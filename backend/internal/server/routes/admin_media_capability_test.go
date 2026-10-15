package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// F01 US1: exercise the production route registration rather than mounting the
// handler directly, so capability CRUD/preview and diagnostics cannot bypass admin auth.
func TestMediaCapabilityAdminRoutesRequireAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{
		Admin:        &handler.AdminHandlers{MediaCapability: adminhandler.NewMediaCapabilityHandler(service.NewMediaCapabilityService(nil))},
		UnifiedMedia: handler.NewUnifiedMediaHandler(nil, nil, nil, nil),
	}
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
			return
		}
		servermiddleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
	})
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, adminAuth, auditLog, stepUp, nil, nil)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/media-capabilities"},
		{http.MethodPost, "/media-capabilities"},
		{http.MethodGet, "/media-capabilities/templates"},
		{http.MethodPost, "/media-capabilities/validate"},
		{http.MethodGet, "/media-capabilities/1"},
		{http.MethodPut, "/media-capabilities/1"},
		{http.MethodDelete, "/media-capabilities/1?revision=1"},
		{http.MethodGet, "/media-tasks"},
		{http.MethodGet, "/media-tasks/media_example"},
	} {
		for _, tc := range []struct {
			name, auth string
			status     int
		}{{"unauthenticated", "", http.StatusUnauthorized}, {"non-admin", "Bearer user-token", http.StatusForbidden}} {
			t.Run(route.method+route.path+"/"+tc.name, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(route.method, "/api/v1/admin"+route.path, nil)
				request.Header.Set("Authorization", tc.auth)
				router.ServeHTTP(recorder, request)
				require.Equal(t, tc.status, recorder.Code)
			})
		}
	}
}
