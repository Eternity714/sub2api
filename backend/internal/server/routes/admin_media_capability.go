package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerMediaCapabilityRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	if h == nil || h.Admin == nil || h.Admin.MediaCapability == nil {
		return
	}
	if h.UnifiedMedia != nil {
		admin.GET("/media-tasks", h.UnifiedMedia.AdminList)
		admin.GET("/media-tasks/:id", h.UnifiedMedia.AdminGet)
	}
	capabilities := admin.Group("/media-capabilities")
	capabilities.GET("", h.Admin.MediaCapability.List)
	capabilities.POST("", h.Admin.MediaCapability.Create)
	capabilities.GET("/templates", h.Admin.MediaCapability.Templates)
	capabilities.POST("/validate", h.Admin.MediaCapability.Validate)
	capabilities.GET("/:id", h.Admin.MediaCapability.Get)
	capabilities.PUT("/:id", h.Admin.MediaCapability.Update)
	capabilities.DELETE("/:id", h.Admin.MediaCapability.Delete)
}
