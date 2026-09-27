package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *GrsaiGatewayHandler) taskCreateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrGrsaiTaskWaitingLimit):
		h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "GRS.AI waiting task limit reached")
	case errors.Is(err, service.ErrGrsaiInsufficientBalance):
		h.errorResponse(c, http.StatusPaymentRequired, "insufficient_balance", "Insufficient balance")
	case errors.Is(err, service.ErrGrsaiSettlementPricingMissing), errors.Is(err, service.ErrGrsaiInvalidRequest), errors.Is(err, service.ErrGrsaiSettlementInvalidInput):
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "GRS.AI request or pricing is invalid")
	default:
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "GRS.AI task creation is unavailable")
	}
}

func (h *GrsaiGatewayHandler) deliverTask(c *gin.Context, task *service.GrsaiSettlement) {
	if task == nil || task.LocalTaskID == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "GRS.AI task is unavailable")
		return
	}
	id := *task.LocalTaskID
	if task.DeliveryMode == string(service.GrsaiDeliveryAsync) {
		c.JSON(http.StatusAccepted, gin.H{"id": id, "status": "queued"})
		return
	}
	stream := task.DeliveryMode == string(service.GrsaiDeliveryStream)
	if stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.SSEvent("task", gin.H{"id": id, "status": "queued", "progress": 0})
		c.Writer.Flush()
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	runningSent := false
	for {
		record, err := h.taskService.GetOwned(c.Request.Context(), task.UserID, task.APIKeyID, id)
		if err != nil {
			if stream {
				c.SSEvent("error", gin.H{"id": id, "error": "Task lookup failed"})
				c.Writer.Flush()
			} else if c.Request.Context().Err() == nil {
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Task lookup failed")
			}
			return
		}
		view := service.NewGrsaiTaskView(record, time.Now().UTC())
		if stream && view.Status == "running" && !runningSent {
			c.SSEvent("task", gin.H{"id": id, "status": "running", "progress": 5})
			c.Writer.Flush()
			runningSent = true
		}
		if view.Status == "succeeded" || view.Status == "failed" || view.Status == "manual_review" {
			if stream {
				if view.Status == "succeeded" && !runningSent {
					c.SSEvent("task", gin.H{"id": id, "status": "running", "progress": 5})
				}
				c.SSEvent("task", view)
				_, _ = c.Writer.WriteString("data: [DONE]\n\n")
				c.Writer.Flush()
			} else {
				status := http.StatusOK
				if view.Status == "failed" {
					status = http.StatusBadGateway
				}
				if view.Status == "manual_review" {
					status = http.StatusServiceUnavailable
				}
				c.JSON(status, view)
			}
			return
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *GrsaiGatewayHandler) taskOwner(c *gin.Context) (int64, int64, bool) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.ID <= 0 {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return 0, 0, false
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 || subject.UserID != apiKey.UserID || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformGrsai {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Task not found")
		return 0, 0, false
	}
	return subject.UserID, apiKey.ID, true
}

func (h *GrsaiGatewayHandler) Result(c *gin.Context) {
	userID, keyID, ok := h.taskOwner(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Query("id"))
	if id == "" {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Task not found")
		return
	}
	if h.taskService == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "GRS.AI tasks are unavailable")
		return
	}
	record, err := h.taskService.GetOwned(c.Request.Context(), userID, keyID, id)
	if errors.Is(err, service.ErrGrsaiSettlementNotFound) {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Task not found")
		return
	}
	if err != nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Task lookup failed")
		return
	}
	c.JSON(http.StatusOK, service.NewGrsaiTaskView(record, time.Now().UTC()))
}

func (h *GrsaiGatewayHandler) Tasks(c *gin.Context) {
	userID, keyID, ok := h.taskOwner(c)
	if !ok {
		return
	}
	if h.taskService == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "GRS.AI tasks are unavailable")
		return
	}
	page, pageSize := 1, 20
	if raw := c.Query("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100000 {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Invalid page")
			return
		}
		page = value
	}
	if raw := c.Query("page_size"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 50 {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Invalid page size")
			return
		}
		pageSize = value
	}
	records, err := h.taskService.ListOwned(c.Request.Context(), userID, keyID, pageSize+1, (page-1)*pageSize)
	if err != nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Task lookup failed")
		return
	}
	hasMore := len(records) > pageSize
	if hasMore {
		records = records[:pageSize]
	}
	views := make([]service.GrsaiTaskView, 0, len(records))
	now := time.Now().UTC()
	for _, record := range records {
		views = append(views, service.NewGrsaiTaskView(record, now))
	}
	c.JSON(http.StatusOK, gin.H{"tasks": views, "page": page, "page_size": pageSize, "has_more": hasMore})
}
