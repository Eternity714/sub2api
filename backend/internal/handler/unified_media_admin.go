package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type unifiedMediaAdminReader interface {
	AdminList(context.Context, int, int, string) ([]*service.UnifiedMediaTask, int64, error)
	GetAdmin(context.Context, string) (*service.UnifiedMediaTask, error)
}

func (h *UnifiedMediaHandler) AdminList(c *gin.Context) {
	reader, ok := h.adminReader()
	if !ok {
		response.Error(c, 503, "媒体任务诊断暂不可用")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	status := c.Query("status")
	if status != "" && status != "queued" && status != "in_progress" && status != "completed" {
		response.Error(c, 400, "status 必须为 queued、in_progress 或 completed")
		return
	}
	tasks, total, err := reader.AdminList(c.Request.Context(), page, size, status)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "媒体任务读取失败")
		return
	}
	items := make([]any, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, unifiedMediaAdminView(task))
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}
func (h *UnifiedMediaHandler) AdminGet(c *gin.Context) {
	reader, ok := h.adminReader()
	if !ok {
		response.Error(c, 503, "媒体任务诊断暂不可用")
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if len(id) > 100 || !strings.HasPrefix(id, "media_") {
		response.Error(c, 404, "媒体任务不存在")
		return
	}
	task, err := reader.GetAdmin(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, unifiedMediaAdminView(task))
}
func (h *UnifiedMediaHandler) adminReader() (unifiedMediaAdminReader, bool) {
	if h == nil {
		return nil, false
	}
	svc, ok := h.tasks.(*service.UnifiedMediaTaskService)
	if !ok || svc == nil {
		return nil, false
	}
	reader, ok := svc.Repo.(unifiedMediaAdminReader)
	return reader, ok
}
func unifiedMediaAdminView(task *service.UnifiedMediaTask) gin.H {
	view := task.PublicView()
	out := gin.H{"id": view.ID, "model": view.Model, "media_type": task.MediaType, "user_id": task.UserID, "api_key_id": task.APIKeyID, "group_id": task.GroupID, "status": view.Status, "progress": view.Progress, "created_at": view.CreatedAt, "held_amount": task.HeldAmount, "charged_amount": task.ChargedAmount, "error": view.Error, "result_count": service.UnifiedMediaTaskSuccessCount(task)}
	if view.CompletedAt != nil {
		out["completed_at"] = *view.CompletedAt
	}
	if view.ExpiresAt != nil {
		out["expires_at"] = *view.ExpiresAt
	}
	units := make([]any, 0, len(task.Units))
	for index, unit := range task.Units {
		units = append(units, gin.H{"index": index, "state": unit.State, "account_id": unit.AccountID, "platform": unit.Platform, "upstream_model": unit.UpstreamModel, "upstream_task_id": unit.UpstreamTaskID, "attempt": unit.Attempt, "count": unit.Count, "stored_count": len(unit.Stored), "last_http_status": unit.LastHTTPStatus, "error": unit.Error})
	}
	out["units"] = units
	return out
}
