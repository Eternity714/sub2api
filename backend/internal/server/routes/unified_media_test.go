package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUnifiedMediaCompositePreservesPublicModelAndCandidates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resolver := service.NewCompositeRouteResolver(compositeRouteRepoStub{routes: []service.CompositeModelRoute{
		{ID: 1, GroupID: 1, PublicModel: "public", UpstreamModel: "nano-banana-2", TargetPlatform: service.PlatformGrsai, Endpoint: "images", MatchType: "exact", Enabled: true},
		{ID: 2, GroupID: 1, PublicModel: "public", UpstreamModel: "gemini-nano-banana-2.1", TargetPlatform: service.PlatformToAPIs, Endpoint: "images", MatchType: "exact", Enabled: true},
		{ID: 3, GroupID: 1, PublicModel: "public", UpstreamModel: "minimax-h3", TargetPlatform: service.PlatformGrsai, Endpoint: "videos", MatchType: "exact", Enabled: true},
	}})
	r := gin.New()
	r.Use(func(c *gin.Context) {
		id := int64(1)
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{GroupID: &id, Group: &service.Group{ID: 1, Platform: service.PlatformComposite}})
	})
	r.Use(compositeTargetPlatformMiddleware(resolver))
	for _, path := range []string{"/v1/images/generations", "/v1/videos/generations"} {
		r.POST(path, func(c *gin.Context) {
			enabled, _ := c.Get("unified_media_protocol")
			require.Equal(t, true, enabled)
			_, resolved := service.ResolvedTargetPlatformFromContext(c.Request.Context())
			require.False(t, resolved)
			raw, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			require.JSONEq(t, `{"model":"public","prompt":"cat"}`, string(raw))
			c.Status(204)
		})
	}
	for _, path := range []string{"/v1/images/generations", "/v1/videos/generations"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"public","prompt":"cat"}`)))
		require.Equal(t, 204, w.Code)
	}
}

// AC-013 (retirement scope): the three standalone GRS routes return 404 without a drain precondition.
func TestUnifiedMediaLegacyGRSEndpointsRetired(t *testing.T) {
	r := newGatewayRoutesTestRouter(service.PlatformGrsai)
	for _, p := range []struct{ method, path string }{{"POST", "/v1/api/generate"}, {"GET", "/v1/api/result"}, {"GET", "/v1/api/tasks"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(p.method, p.path, nil))
		require.Equal(t, 404, w.Code, p.path)
	}
}

func TestUnifiedMediaOpenAIOptInKeepsMixedLegacyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resolver := service.NewCompositeRouteResolver(compositeRouteRepoStub{routes: []service.CompositeModelRoute{
		{ID: 1, GroupID: 1, PublicModel: "new-image", UpstreamModel: "gpt-image-2.6-1k", TargetPlatform: service.PlatformOpenAI, Endpoint: "images", MatchType: "exact", CapabilityModel: "new-image", Enabled: true},
		{ID: 2, GroupID: 1, PublicModel: "legacy-image", UpstreamModel: "legacy-upstream", TargetPlatform: service.PlatformOpenAI, Endpoint: "images", MatchType: "exact", Enabled: true},
		{ID: 3, GroupID: 1, PublicModel: "legacy-video", UpstreamModel: "video-upstream", TargetPlatform: service.PlatformGrok, Endpoint: "videos", MatchType: "exact", Enabled: true},
	}})
	for _, tc := range []struct {
		model, endpoint, upstream string
		unified                   bool
	}{
		{"new-image", "images", "", true},
		{"legacy-image", "images", "legacy-upstream", false},
		{"legacy-video", "videos", "video-upstream", false},
	} {
		t.Run(tc.model, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				id := int64(1)
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{GroupID: &id, Group: &service.Group{ID: 1, Platform: service.PlatformComposite}})
			})
			r.Use(compositeTargetPlatformMiddleware(resolver))
			path := "/v1/" + tc.endpoint + "/generations"
			r.POST(path, func(c *gin.Context) {
				enabled, _ := c.Get("unified_media_protocol")
				require.Equal(t, tc.unified, enabled == true)
				raw, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				expectedModel := tc.upstream
				if tc.unified {
					expectedModel = tc.model
				}
				require.Contains(t, string(raw), expectedModel)
				c.Status(204)
			})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"`+tc.model+`","prompt":"cup"}`))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			require.Equal(t, 204, w.Code)
		})
	}
}
