package handler

import (
	"bytes"
	"context"
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
	gatewayService           grsaiAccountSelector
	billingCacheService      grsaiBillingEligibility
	nativeClient             service.GrsaiNativeClient
	settlementService        *service.GrsaiSettlementService
	taskService              *service.GrsaiTaskService
	contentModerationService *service.ContentModerationService
	securityAuditCoordinator *securityaudit.Coordinator
	concurrencyHelper        *ConcurrencyHelper
}

type grsaiAccountSelector interface {
	SelectAccountWithLoadAwareness(context.Context, *int64, string, string, map[int64]struct{}, string, int64) (*service.AccountSelectionResult, error)
	ResolveUserGroupRateMultiplier(context.Context, int64, int64, float64) float64
}

type grsaiBillingEligibility interface {
	CheckBillingEligibility(context.Context, *service.User, *service.APIKey, *service.Group, *service.UserSubscription, string) error
}

// SetTaskService wires the durable delivery state machine. It is kept as a
// setter so existing handler constructors and tests remain source-compatible;
// production wiring supplies it during Task 7 lifecycle setup.
func (h *GrsaiGatewayHandler) SetTaskService(tasks *service.GrsaiTaskService) {
	if h != nil {
		h.taskService = tasks
	}
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
	if h.gatewayService == nil || h.billingCacheService == nil || h.nativeClient == nil || h.settlementService == nil || h.taskService == nil || h.concurrencyHelper == nil {
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
	model, imageCount, imageSize := parseGrsaiGenerateRequest(preparedBody)
	if model == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	delivery, err := service.ParseGrsaiDeliveryRequest(body)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if delivery.Mode == service.GrsaiDeliveryStream && !h.taskService.Options.StreamEnabled {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "GRS.AI stream delivery is unavailable")
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
		// Streaming requests continue durable upstream processing after an SSE
		// client disconnects, so the user slot must stay held until that work ends.
		defer userRelease()
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
	if account.Platform != service.PlatformGrsai || account.Type != service.AccountTypeAPIKey {
		if selection.ReleaseFunc != nil {
			wrapReleaseOnDone(c.Request.Context(), selection.ReleaseFunc)()
		}
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available GRS.AI accounts")
		return
	}
	setOpsSelectedAccount(c, account.ID, account.Platform)

	accountRelease, acquired := h.acquireAccountSlot(c, selection, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if accountRelease != nil {
		// See the user-slot note above: releasing on the request context's
		// cancellation would allow another request to exceed account concurrency
		// while this durable stream is still active.
		defer accountRelease()
	}

	groupRate := h.gatewayService.ResolveUserGroupRateMultiplier(c.Request.Context(), apiKey.UserID, apiKey.Group.ID, apiKey.Group.RateMultiplier)
	if delivery.Mode == service.GrsaiDeliveryAsync {
		if h.taskService == nil || !h.taskService.Options.Enabled {
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "GRS.AI async delivery is unavailable")
			return
		}
		task, err := h.taskService.CreateGrsaiTask(c.Request.Context(), service.GrsaiTaskInput{
			Account: account, APIKey: apiKey, Body: body, Request: delivery,
			EffectiveGroupMultiplier: &groupRate,
		})
		if err != nil {
			reqLog.Warn("grsai.async_task_create_failed", zap.Error(err))
			if errors.Is(err, service.ErrGrsaiAsyncQueueFull) {
				h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending GRS.AI tasks")
				return
			}
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Unable to create GRS.AI task")
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"id": task.PublicTaskID, "status": "queued", "model": task.Model, "created_at": task.CreatedAt, "updated_at": task.UpdatedAt})
		return
	}
	settlement, err := h.settlementService.Prepare(c.Request.Context(), service.GrsaiPrepareInput{
		Account: account, APIKey: apiKey, Model: model, ImageCount: imageCount, ImageSize: imageSize, EffectiveGroupMultiplier: &groupRate, DeliveryMode: delivery.Mode,
	})
	if err != nil {
		reqLog.Warn("grsai.settlement_prepare_failed", zap.Error(err))
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "GRS.AI model pricing is unavailable")
		return
	}

	if delivery.Mode == service.GrsaiDeliveryJSON || delivery.Mode == service.GrsaiDeliveryStream {
		settlement.OriginalBody = append([]byte(nil), delivery.OriginalBody...)
		settlement.UpstreamBody = append([]byte(nil), delivery.UpstreamBody...)
		streamCtx := context.WithoutCancel(c.Request.Context())
		var firstWrite bool
		var downstreamDisconnected bool
		final, runErr := h.taskService.RunGrsaiTask(streamCtx, settlement, func(event service.GrsaiStreamEvent) error {
			if delivery.Mode == service.GrsaiDeliveryStream {
				if downstreamDisconnected || c.Request.Context().Err() != nil {
					downstreamDisconnected = true
					return nil
				}
				if !firstWrite {
					c.Header("Content-Type", "text/event-stream")
					c.Header("Cache-Control", "no-cache")
					c.Header("Connection", "keep-alive")
					c.Writer.WriteHeader(http.StatusOK)
					firstWrite = true
				}
				if _, err := c.Writer.Write([]byte("data: " + string(event.RawData) + "\n\n")); err != nil {
					downstreamDisconnected = true
					return nil
				}
				if flusher, ok := c.Writer.(http.Flusher); ok {
					flusher.Flush()
				}
				return nil
			}
			return nil
		})
		if delivery.Mode == service.GrsaiDeliveryStream {
			// Once SSE headers or a frame have been written, the response cannot
			// be changed to JSON. A client disconnect is an output concern only;
			// the durable task service has already recorded the provider event and
			// will continue recovery without resubmitting it.
			if runErr != nil && !firstWrite && !errors.Is(runErr, context.Canceled) {
				h.errorResponse(c, http.StatusBadGateway, "api_error", "GRS.AI upstream stream failed")
			}
			return
		}
		h.writeGrsaiJSONRunResult(c, final, runErr)
		return
	}
	h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "GRS.AI delivery mode is unavailable")
}

func (h *GrsaiGatewayHandler) writeGrsaiJSONRunResult(c *gin.Context, final *service.GrsaiUpstreamResult, runErr error) {
	// RunGrsaiTask returns a verified terminal result even when durable billing
	// is pending retry. Do not turn a successful generation into a retryable 502.
	if final != nil && final.TaskID != "" && (final.Status == service.GrsaiUpstreamStatusSucceeded ||
		final.Status == service.GrsaiUpstreamStatusFailed || final.Status == service.GrsaiUpstreamStatusViolation) {
		h.writeGrsaiTerminalJSON(c, final)
		return
	}
	if runErr != nil {
		h.errorResponse(c, http.StatusBadGateway, "api_error", "GRS.AI upstream request failed")
		return
	}
	h.errorResponse(c, http.StatusBadGateway, "api_error", "GRS.AI upstream returned an invalid response")
}

func (h *GrsaiGatewayHandler) writeGrsaiTerminalJSON(c *gin.Context, result *service.GrsaiUpstreamResult) {
	if result == nil {
		h.errorResponse(c, http.StatusBadGateway, "api_error", "GRS.AI upstream returned an invalid response")
		return
	}
	status := result.HTTPStatus
	if status < 200 || status >= 300 {
		status = http.StatusBadGateway
	}
	body := gin.H{"id": result.TaskID, "status": result.Status, "progress": result.Progress}
	if len(result.ResultURLs) > 0 {
		body["results"] = result.ResultURLs
	}
	if result.ErrorCode != "" {
		body["error_code"] = result.ErrorCode
	}
	if result.ErrorMessage != "" {
		body["error_message"] = result.ErrorMessage
	}
	c.JSON(status, body)
}

// Result returns an owner-scoped public task view by local or upstream ID.
func (h *GrsaiGatewayHandler) Result(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformGrsai || h.taskService == nil {
		h.errorResponse(c, http.StatusNotFound, "not_found", "task not found")
		return
	}
	view, err := h.taskService.GetPublicTaskView(c.Request.Context(), subject.UserID, apiKey.ID, c.Query("id"))
	if err != nil || view == nil {
		h.errorResponse(c, http.StatusNotFound, "not_found", "task not found")
		return
	}
	c.JSON(http.StatusOK, view)
}

// acquireAccountSlot honors the scheduler's immediate lease or its bounded
// WaitPlan. A native GRS.AI request must not create a settlement or reach the
// upstream until this returns an acquired account slot.
func (h *GrsaiGatewayHandler) acquireAccountSlot(
	c *gin.Context,
	selection *service.AccountSelectionResult,
	streamStarted *bool,
	reqLog *zap.Logger,
) (func(), bool) {
	if selection == nil || selection.Account == nil {
		markOpsRoutingCapacityLimited(c)
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available GRS.AI accounts")
		return nil, false
	}
	if selection.Acquired {
		return selection.ReleaseFunc, true
	}
	if selection.WaitPlan == nil {
		markOpsRoutingCapacityLimited(c)
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available GRS.AI accounts")
		return nil, false
	}

	account := selection.Account
	accountWaitCounted := false
	canWait, waitErr := h.concurrencyHelper.IncrementAccountWaitCount(c.Request.Context(), account.ID, selection.WaitPlan.MaxWaiting)
	if waitErr != nil {
		reqLog.Warn("grsai.account_wait_counter_increment_failed", zap.Int64("account_id", account.ID), zap.Error(waitErr))
	} else if !canWait {
		reqLog.Info("grsai.account_wait_queue_full", zap.Int64("account_id", account.ID), zap.Int("max_waiting", selection.WaitPlan.MaxWaiting))
		h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending requests, please retry later")
		return nil, false
	} else {
		accountWaitCounted = true
	}
	releaseWait := func() {
		if accountWaitCounted {
			h.concurrencyHelper.DecrementAccountWaitCount(c.Request.Context(), account.ID)
			accountWaitCounted = false
		}
	}
	defer releaseWait()

	release, err := h.concurrencyHelper.AcquireAccountSlotWithWaitTimeout(
		c,
		account.ID,
		selection.WaitPlan.MaxConcurrency,
		selection.WaitPlan.Timeout,
		false,
		streamStarted,
	)
	if err != nil {
		reqLog.Warn("grsai.account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		status, errType, _, message := concurrencyErrorResponse(err, "account")
		h.errorResponse(c, status, errType, message)
		return nil, false
	}
	return release, true
}

// redactGrsaiUpstreamBody is intentionally byte-preserving except for exact
// occurrences of the selected account credential. Providers sometimes echo
// credentials in JSON and sometimes in plain-text error bodies.
func redactGrsaiUpstreamBody(raw []byte, account *service.Account) []byte {
	if len(raw) == 0 || account == nil {
		return raw
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return raw
	}
	return bytes.ReplaceAll(raw, []byte(apiKey), []byte("REDACTED"))
}

func (h *GrsaiGatewayHandler) checkSecurityAudit(c *gin.Context, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, model string, body []byte) *securityaudit.Decision {
	if h == nil {
		return nil
	}
	return runSecurityAudit(c, reqLog, h.securityAuditCoordinator, h.contentModerationService, apiKey, subject, service.ContentModerationProtocolOpenAIImages, model, body, "http")
}

func parseGrsaiGenerateRequest(body []byte) (string, int, string) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return "", 1, service.ImageBillingSize2K
	}
	var model string
	if json.Unmarshal(fields["model"], &model) != nil {
		return "", 1, service.ImageBillingSize2K
	}
	model = strings.TrimSpace(model)
	imageCount := 1
	for _, key := range []string{"n", "numImages", "num_images", "imageCount", "image_count"} {
		var count int
		if raw, exists := fields[key]; exists && json.Unmarshal(raw, &count) == nil && count > 0 {
			imageCount = count
			break
		}
	}
	var rawImageSize string
	_ = json.Unmarshal(fields["imageSize"], &rawImageSize)
	return model, imageCount, service.NormalizeImageBillingTierOrDefault(rawImageSize)
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
