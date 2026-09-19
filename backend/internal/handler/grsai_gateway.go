package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GrsaiGatewayHandler handles the native GRS.AI image generation protocol.
// It is intentionally separate from the OpenAI images handler because native
// fields and terminal task semantics must not be translated.
type GrsaiGatewayHandler struct {
	gatewayService           *service.GatewayService
	billingCacheService      *service.BillingCacheService
	nativeClient             service.GrsaiNativeClient
	settlementService        *service.GrsaiSettlementService
	contentModerationService *service.ContentModerationService
	securityAuditCoordinator *securityaudit.Coordinator
	concurrencyHelper        *ConcurrencyHelper
}

func NewGrsaiGatewayHandler(
	gatewayService *service.GatewayService,
	concurrencyService *service.ConcurrencyService,
	billingCacheService *service.BillingCacheService,
	nativeClient service.GrsaiNativeClient,
	settlementService *service.GrsaiSettlementService,
	contentModerationService *service.ContentModerationService,
) *GrsaiGatewayHandler {
	return &GrsaiGatewayHandler{
		gatewayService:           gatewayService,
		billingCacheService:      billingCacheService,
		nativeClient:             nativeClient,
		settlementService:        settlementService,
		contentModerationService: contentModerationService,
		concurrencyHelper:        NewConcurrencyHelper(concurrencyService, SSEPingFormatNone, 0),
	}
}

// Generate handles POST /v1/api/generate. A request is sent upstream exactly
// once: after the durable pre-submission record exists, no retry/failover path
// is allowed in this handler.
func (h *GrsaiGatewayHandler) Generate(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	if apiKey.Group == nil || apiKey.Group.Platform != service.PlatformGrsai {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "GRS.AI native images are not supported for this group")
		return
	}
	if h.gatewayService == nil || h.billingCacheService == nil || h.nativeClient == nil || h.settlementService == nil || h.concurrencyHelper == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "GRS.AI gateway is unavailable")
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", "Request body is too large")
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	preparedBody, err := service.PrepareGrsaiGenerateBody(body)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	model, imageCount := parseGrsaiGenerateRequest(preparedBody)
	if model == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}

	reqLog := requestLogger(c, "handler.grsai_gateway.generate",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID), zap.Any("group_id", apiKey.GroupID),
		zap.String("model", model), zap.Int("image_count", imageCount))
	setOpsRequestContext(c, model, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))
	if !service.GroupAllowsImageGeneration(apiKey.Group) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, model, preparedBody); decision != nil && !decision.AllowNextStage {
		h.securityAuditError(c, decision)
		return
	}

	streamStarted := false
	userRelease, err := h.concurrencyHelper.AcquireUserSlotWithWait(c, subject.UserID, subject.Concurrency, false, &streamStarted)
	if err != nil {
		reqLog.Warn("grsai.user_slot_acquire_failed", zap.Error(err))
		h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Concurrency limit reached")
		return
	}
	if userRelease != nil {
		defer wrapReleaseOnDone(c.Request.Context(), userRelease)()
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.errorResponse(c, status, code, message)
		return
	}

	selection, err := h.gatewayService.SelectAccountWithLoadAwareness(c.Request.Context(), apiKey.GroupID, "", model, nil, "", subject.UserID)
	if err != nil || selection == nil || selection.Account == nil {
		if err != nil {
			reqLog.Warn("grsai.account_select_failed", zap.Error(err))
		}
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available GRS.AI accounts")
		return
	}
	account := selection.Account
	if selection.ReleaseFunc != nil {
		defer wrapReleaseOnDone(c.Request.Context(), selection.ReleaseFunc)()
	}
	if account.Platform != service.PlatformGrsai || account.Type != service.AccountTypeAPIKey {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available GRS.AI accounts")
		return
	}
	setOpsSelectedAccount(c, account.ID, account.Platform)

	groupRate := h.gatewayService.ResolveUserGroupRateMultiplier(c.Request.Context(), apiKey.UserID, apiKey.Group.ID, apiKey.Group.RateMultiplier)
	settlement, err := h.settlementService.Prepare(c.Request.Context(), service.GrsaiPrepareInput{
		Account: account, APIKey: apiKey, Model: model, ImageCount: imageCount, EffectiveGroupMultiplier: &groupRate,
	})
	if err != nil {
		reqLog.Warn("grsai.settlement_prepare_failed", zap.Error(err))
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "GRS.AI model pricing is unavailable")
		return
	}

	upstream, upstreamErr := h.nativeClient.Generate(c.Request.Context(), account, preparedBody)
	outcome := h.settlementService.Finish(c.Request.Context(), settlement, upstream, upstreamErr)
	if outcome != nil && outcome.SettlementError != nil && !errors.Is(outcome.SettlementError, service.ErrGrsaiSettlementClaimLost) {
		// The upstream outcome has already been determined. Returning it avoids a
		// client retry that could create a second provider task.
		reqLog.Error("grsai.settlement_finish_failed", zap.String("priority", "high"), zap.Int64("settlement_id", settlement.ID), zap.Error(outcome.SettlementError))
	}

	var upstreamHTTPError *service.GrsaiHTTPError
	if upstream != nil && len(upstream.RawBody) > 0 && (upstreamErr == nil || errors.As(upstreamErr, &upstreamHTTPError)) {
		status := upstream.HTTPStatus
		if status == 0 {
			status = http.StatusBadGateway
		}
		c.Data(status, "application/json", upstream.RawBody)
		return
	}
	if upstreamErr != nil {
		reqLog.Warn("grsai.upstream_request_failed", zap.Error(upstreamErr))
		h.errorResponse(c, http.StatusBadGateway, "api_error", "GRS.AI upstream request failed")
		return
	}
	h.errorResponse(c, http.StatusBadGateway, "api_error", "GRS.AI upstream returned an invalid response")
}

func (h *GrsaiGatewayHandler) checkSecurityAudit(c *gin.Context, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, model string, body []byte) *securityaudit.Decision {
	if h == nil {
		return nil
	}
	return runSecurityAudit(c, reqLog, h.securityAuditCoordinator, h.contentModerationService, apiKey, subject, service.ContentModerationProtocolOpenAIImages, model, body, "http")
}

func parseGrsaiGenerateRequest(body []byte) (string, int) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return "", 1
	}
	var model string
	if json.Unmarshal(fields["model"], &model) != nil {
		return "", 1
	}
	model = strings.TrimSpace(model)
	for _, key := range []string{"n", "numImages", "num_images", "imageCount", "image_count"} {
		var count int
		if raw, exists := fields[key]; exists && json.Unmarshal(raw, &count) == nil && count > 0 {
			return model, count
		}
	}
	return model, 1
}

func (h *GrsaiGatewayHandler) errorResponse(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{"error": gin.H{"type": errType, "message": message}})
}

func (h *GrsaiGatewayHandler) securityAuditError(c *gin.Context, decision *securityaudit.Decision) {
	if decision == nil {
		return
	}
	errType := "api_error"
	if decision.Kind == securityaudit.DecisionBlock {
		errType = "permission_error"
	}
	h.errorResponse(c, securityAuditStatus(decision), errType, securityAuditMessage(decision))
}
