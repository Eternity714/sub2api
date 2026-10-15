package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/mediacapability"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type unifiedMediaTasks interface {
	Create(context.Context, service.UnifiedMediaTaskCreateInput) (*service.UnifiedMediaTask, error)
	GetOwned(context.Context, int64, int64, string, string) (*service.UnifiedMediaTask, error)
}
type UnifiedMediaHandler struct {
	tasks   unifiedMediaTasks
	gateway *service.GatewayService
	billing *service.BillingCacheService
	audit   *GrsaiGatewayHandler
}

func NewUnifiedMediaHandler(tasks *service.UnifiedMediaTaskService, gateway *service.GatewayService, billing *service.BillingCacheService,
	audit *GrsaiGatewayHandler) *UnifiedMediaHandler {
	return &UnifiedMediaHandler{tasks: tasks, gateway: gateway, billing: billing, audit: audit}
}
func (h *UnifiedMediaHandler) CreateImage(c *gin.Context) { h.create(c, "image") }
func (h *UnifiedMediaHandler) CreateVideo(c *gin.Context) { h.create(c, "video") }
func (h *UnifiedMediaHandler) GetImage(c *gin.Context)    { h.get(c, "image") }
func (h *UnifiedMediaHandler) GetVideo(c *gin.Context)    { h.get(c, "video") }
func (h *UnifiedMediaHandler) create(c *gin.Context, kind string) {
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || key == nil {
		unifiedMediaHTTPError(c, http.StatusUnauthorized, "authentication_error", "Invalid API key", nil)
		return
	}
	if key.Group == nil || key.Group.Platform != service.PlatformComposite {
		unifiedMediaHTTPError(c, 404, "not_found_error", "Media API is unavailable for this group", nil)
		return
	}
	if !service.GroupAllowsImageGeneration(key.Group) {
		unifiedMediaHTTPError(c, 403, "permission_error", service.ImageGenerationPermissionMessage(), nil)
		return
	}
	if h == nil || h.tasks == nil {
		unifiedMediaHTTPError(c, 503, "api_error", "Media generation is unavailable", nil)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, mediacapability.MaxRequestBytes+1))
	if err != nil || len(raw) > mediacapability.MaxRequestBytes {
		unifiedMediaHTTPError(c, 400, "invalid_request_error", "Media request body is invalid or too large", nil)
		return
	}
	var envelope struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(raw, &envelope) == nil && h.audit != nil {
		if subject, exists := middleware.GetAuthSubjectFromContext(c); exists {
			decision := h.audit.checkSecurityAudit(c, requestLogger(c, "handler.unified_media.create"), key, subject, envelope.Model, raw)
			if decision != nil && !decision.AllowNextStage {
				h.audit.securityAuditError(c, decision)
				return
			}
		}
	}
	var rate *float64
	if h.gateway != nil {
		v := h.gateway.ResolveUserGroupRateMultiplier(c.Request.Context(), key.UserID, key.Group.ID, key.Group.RateMultiplier)
		rate = &v
	}
	task, err := h.tasks.Create(c.Request.Context(), service.UnifiedMediaTaskCreateInput{APIKey: key, MediaType: kind, Body: raw, EffectiveGroupMultiplier: rate})
	if err != nil {
		writeUnifiedMediaError(c, err)
		return
	}
	if task == nil {
		unifiedMediaHTTPError(c, 503, "api_error", "Media task creation failed", nil)
		return
	}
	if h.billing != nil {
		_ = h.billing.InvalidateUserBalance(c.Request.Context(), key.UserID)
	}
	c.JSON(http.StatusAccepted, gin.H{"id": task.ID, "model": task.Model, "status": "queued", "progress": 0, "created_at": task.CreatedAt.Unix()})
}
func (h *UnifiedMediaHandler) get(c *gin.Context, kind string) {
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || key == nil {
		unifiedMediaHTTPError(c, 401, "authentication_error", "Invalid API key", nil)
		return
	}
	if key.Group == nil || key.Group.Platform != service.PlatformComposite {
		unifiedMediaHTTPError(c, 404, "not_found_error", "Media task not found", nil)
		return
	}
	id := strings.TrimSpace(c.Param("task_id"))
	if id == "" {
		id = strings.TrimSpace(c.Param("request_id"))
	}
	if len(id) > 100 || !strings.HasPrefix(id, "media_") {
		unifiedMediaHTTPError(c, 404, "not_found_error", "Media task not found", nil)
		return
	}
	if h == nil || h.tasks == nil {
		unifiedMediaHTTPError(c, 503, "api_error", "Media task lookup is unavailable", nil)
		return
	}
	task, err := h.tasks.GetOwned(c.Request.Context(), key.UserID, key.ID, kind, id)
	if err != nil {
		writeUnifiedMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, task.PublicView())
}
func writeUnifiedMediaError(c *gin.Context, err error) {
	status := infraerrors.Code(err)
	message := infraerrors.Message(err)
	if status < 400 || status > 599 {
		status = 500
	}
	if status >= 500 {
		message = "Media service is temporarily unavailable"
	}
	var fields []mediacapability.FieldError
	app := infraerrors.FromError(err)
	if app != nil && app.Metadata != nil {
		_ = json.Unmarshal([]byte(app.Metadata["fields"]), &fields)
	}
	unifiedMediaHTTPError(c, status, infraerrors.Reason(err), message, fields)
}
func unifiedMediaHTTPError(c *gin.Context, status int, code, message string, fields []mediacapability.FieldError) {
	detail := gin.H{"type": code, "message": message}
	if len(fields) > 0 {
		detail["fields"] = fields
	}
	c.JSON(status, gin.H{"error": detail})
}
