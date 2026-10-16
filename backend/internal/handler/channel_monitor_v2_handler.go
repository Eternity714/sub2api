package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ChannelMonitorV2Handler struct {
	service       *service.ChannelMonitorV2Service
	apiKeyService channelMonitorV2GroupAuthorizer
}

type channelMonitorV2GroupAuthorizer interface {
	GetAvailableGroups(ctx context.Context, userID int64) ([]service.Group, error)
}

func NewChannelMonitorV2Handler(svc *service.ChannelMonitorV2Service, apiKeyService *service.APIKeyService) *ChannelMonitorV2Handler {
	return &ChannelMonitorV2Handler{service: svc, apiKeyService: apiKeyService}
}

// channelMonitorV2IsAdmin is true when the request already passed admin auth
// (shared Dimensions/Errors handlers serve both user and admin route groups).
func channelMonitorV2IsAdmin(c *gin.Context) bool {
	role, ok := middleware.GetUserRoleFromContext(c)
	return ok && role == service.RoleAdmin
}

func (h *ChannelMonitorV2Handler) GetConfig(c *gin.Context) {
	cfg, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ChannelMonitorV2Handler) UpdateConfig(c *gin.Context) {
	var input service.ChannelMonitorV2Config
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid channel monitor v2 config")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "user not found in context")
		return
	}
	updated, err := h.service.UpdateConfig(c.Request.Context(), input, input.Version, subject.UserID)
	if err != nil {
		if errors.Is(err, service.ErrChannelMonitorV2ConfigConflict) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, service.ErrChannelMonitorV2InvalidConfig) {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *ChannelMonitorV2Handler) Dimensions(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	admin := channelMonitorV2IsAdmin(c)
	if !h.scopeFilter(c, &filter, admin) {
		return
	}
	result, err := h.service.Dimensions(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// Admin and user share this handler; only non-admin responses strip volume.
	if !admin {
		result = userMonitorV2Dimensions(result)
		service.RedactChannelMonitorV2Dimensions(result)
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) Snapshot(c *gin.Context)      { h.snapshot(c, false) }
func (h *ChannelMonitorV2Handler) AdminSnapshot(c *gin.Context) { h.snapshot(c, true) }
func (h *ChannelMonitorV2Handler) Models(c *gin.Context)        { h.models(c, false) }
func (h *ChannelMonitorV2Handler) AdminModels(c *gin.Context)   { h.models(c, true) }
func (h *ChannelMonitorV2Handler) Matrix(c *gin.Context)        { h.matrix(c, false) }
func (h *ChannelMonitorV2Handler) AdminMatrix(c *gin.Context)   { h.matrix(c, true) }
func (h *ChannelMonitorV2Handler) Users(c *gin.Context)         { h.users(c, false) }
func (h *ChannelMonitorV2Handler) AdminUsers(c *gin.Context)    { h.users(c, true) }

func (h *ChannelMonitorV2Handler) snapshot(c *gin.Context, admin bool) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	if !h.scopeFilter(c, &filter, admin) {
		return
	}
	result, err := h.service.Snapshot(c.Request.Context(), filter, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !admin && result != nil {
		out := *result
		out.Config.Platforms = userMonitorV2PlatformConfig(result.Config.Platforms)
		result = &out
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) models(c *gin.Context, admin bool) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	if !h.scopeFilter(c, &filter, admin) {
		return
	}
	result, err := h.service.Models(c.Request.Context(), filter, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !admin && result != nil {
		out := *result
		out.Items = append([]service.ChannelMonitorV2ModelRow(nil), result.Items...)
		for i := range out.Items {
			out.Items[i].Platform = dto.UserVisiblePlatform(out.Items[i].Platform)
		}
		result = &out
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) matrix(c *gin.Context, admin bool) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	groupBy, err := service.ParseChannelMonitorV2GroupBy(c.Query("group_by"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if !h.scopeFilter(c, &filter, admin) {
		return
	}
	result, err := h.service.Matrix(c.Request.Context(), filter, groupBy, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !admin && result != nil {
		out := *result
		out.Items = append([]service.ChannelMonitorV2MatrixRow(nil), result.Items...)
		for i := range out.Items {
			out.Items[i].Platform = dto.UserVisiblePlatform(out.Items[i].Platform)
		}
		result = &out
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) Errors(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	admin := channelMonitorV2IsAdmin(c)
	if !h.scopeFilter(c, &filter, admin) {
		return
	}
	result, err := h.service.ErrorsForViewer(c.Request.Context(), filter, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) users(c *gin.Context, admin bool) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	subject, exists := middleware.GetAuthSubjectFromContext(c)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "user not found in context")
		return
	}
	if !h.scopeFilter(c, &filter, admin) {
		return
	}
	result, err := h.service.Users(c.Request.Context(), filter, subject.UserID, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ChannelMonitorV2Handler) scopeFilter(c *gin.Context, filter *service.ChannelMonitorV2Filter, admin bool) bool {
	if admin {
		return true
	}
	if h.apiKeyService == nil {
		response.Error(c, http.StatusInternalServerError, "channel monitor group authorization unavailable")
		return false
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "user not found in context")
		return false
	}
	groups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	filter.RestrictGroups = true
	filter.Platforms = expandUserMonitorV2MediaFilter(filter.Platforms, "")
	filter.Models = expandUserMonitorV2MediaFilter(filter.Models, "\x00")
	filter.AllowedGroupIDs = make([]int64, 0, len(groups))
	for i := range groups {
		filter.AllowedGroupIDs = append(filter.AllowedGroupIDs, groups[i].ID)
	}
	return true
}

func (h *ChannelMonitorV2Handler) parseFilter(c *gin.Context) (service.ChannelMonitorV2Filter, bool) {
	groups, err := parseChannelMonitorV2GroupIDs(queryList(c, "group_id"))
	if err != nil {
		response.BadRequest(c, "invalid group_id")
		return service.ChannelMonitorV2Filter{}, false
	}
	filter, err := h.service.ParseFilter(c.Query("range"), queryList(c, "platform"), queryList(c, "model"), groups)
	if err != nil {
		response.BadRequest(c, err.Error())
		return service.ChannelMonitorV2Filter{}, false
	}
	return filter, true
}

func queryList(c *gin.Context, key string) []string {
	values := c.QueryArray(key)
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

func userMonitorV2Dimensions(src *service.ChannelMonitorV2Dimensions) *service.ChannelMonitorV2Dimensions {
	if src == nil {
		return nil
	}
	out := *src
	out.Platforms = make([]service.ChannelMonitorV2Dimension, 0, len(src.Platforms))
	out.Groups = append([]service.ChannelMonitorV2GroupDimension(nil), src.Groups...)
	out.Models = make([]service.ChannelMonitorV2Dimension, 0, len(src.Models))
	platforms := make(map[string]int, len(src.Platforms))
	for _, dimension := range src.Platforms {
		if dto.UserVisiblePlatform(dimension.Value) == "media" {
			dimension.Value = "media"
			dimension.Label = "Media API"
			dimension.Platform = "media"
		}
		dimension.Platform = dto.UserVisiblePlatform(dimension.Platform)
		if index, exists := platforms[dimension.Value]; exists {
			out.Platforms[index].RequestCount += dimension.RequestCount
		} else {
			platforms[dimension.Value] = len(out.Platforms)
			out.Platforms = append(out.Platforms, dimension)
		}
	}
	for i := range out.Groups {
		out.Groups[i].Platform = dto.UserVisiblePlatform(out.Groups[i].Platform)
	}
	models := make(map[string]int, len(src.Models))
	for _, dimension := range src.Models {
		if platform, model, ok := strings.Cut(dimension.Value, "\x00"); ok && dto.UserVisiblePlatform(platform) == "media" {
			dimension.Value = "media\x00" + model
			dimension.Label = model
			dimension.Platform = "media"
			if model == service.ChannelMonitorV2OtherModel {
				dimension.Label = "Other models"
			}
		}
		dimension.Platform = dto.UserVisiblePlatform(dimension.Platform)
		if index, exists := models[dimension.Value]; exists {
			out.Models[index].RequestCount += dimension.RequestCount
		} else {
			models[dimension.Value] = len(out.Models)
			out.Models = append(out.Models, dimension)
		}
	}
	return &out
}

func expandUserMonitorV2MediaFilter(src []string, separator string) []string {
	out := make([]string, 0, len(src))
	seen := make(map[string]bool, len(src))
	appendValue := func(value string) {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	for _, value := range src {
		model, media := strings.CutPrefix(value, "media"+separator)
		if media && (separator != "" || model == "") {
			appendValue("grsai" + separator + model)
			appendValue("toapis" + separator + model)
		} else {
			appendValue(value)
		}
	}
	return out
}

func userMonitorV2PlatformConfig(src []service.ChannelMonitorV2PlatformConfig) []service.ChannelMonitorV2PlatformConfig {
	out := make([]service.ChannelMonitorV2PlatformConfig, 0, len(src))
	indexes := make(map[string]int, len(src))
	for _, platform := range src {
		platform.Platform = dto.UserVisiblePlatform(platform.Platform)
		// Public snapshots expose availability only; the service strips model allow-lists.
		platform.Models = nil
		if index, exists := indexes[platform.Platform]; exists {
			out[index].Enabled = out[index].Enabled || platform.Enabled
		} else {
			indexes[platform.Platform] = len(out)
			out = append(out, platform)
		}
	}
	return out
}

func parseChannelMonitorV2GroupIDs(values []string) ([]int64, error) {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("invalid group id")
		}
		result = append(result, id)
	}
	return result, nil
}
