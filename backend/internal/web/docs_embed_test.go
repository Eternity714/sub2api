//go:build embed

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedDocumentationEntrypoints(t *testing.T) {
	for _, entrypoint := range []string{"settings", "legacy"} {
		t.Run(entrypoint, func(t *testing.T) {
			distFS := documentationTestFS()
			var frontendMiddleware gin.HandlerFunc
			if entrypoint == "settings" {
				provider := &mockSettingsProvider{settings: map[string]string{"site_name": "Gkotta"}}
				server, err := NewFrontendServer(provider)
				require.NoError(t, err)
				server.distFS = distFS
				server.baseHTML = []byte("<html><head></head><body>main-spa</body></html>")
				server.overrideDir = ""
				server.cache.SetBaseHTML(server.baseHTML)
				server.InvalidateCache()
				frontendMiddleware = server.Middleware()
			} else {
				frontendMiddleware = serveEmbeddedFrontendFromFS(distFS, "")
			}
			router := gin.New()
			router.Use(middleware.SecurityHeaders(config.CSPConfig{Enabled: true}, nil), frontendMiddleware)
			for _, tc := range []struct {
				path   string
				status int
				body   string
			}{
				{"/docs", http.StatusPermanentRedirect, ""},
				{"/docs/", http.StatusOK, "docs-home"},
				{"/docs/quick-start", http.StatusOK, "docs-quick-start"},
				{"/docs/assets/app.AbCd1234.js", http.StatusOK, "export const docs"},
				{"/docs/missing", http.StatusNotFound, "docs-not-found"},
				{"/docs/batch-image", http.StatusOK, "main-spa"},
			} {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
				assert.Equal(t, tc.status, w.Code, tc.path)
				assert.Contains(t, w.Body.String(), tc.body, tc.path)
				if tc.path == "/docs/" || tc.path == "/docs/quick-start" || tc.path == "/docs/missing" {
					assert.Contains(t, w.Body.String(), `nonce="`, tc.path)
					assert.Equal(t, "no-store", w.Header().Get("Cache-Control"), tc.path)
					assert.NotContains(t, w.Body.String(), "main-spa", tc.path)
				}
				if strings.HasSuffix(tc.path, ".js") {
					assert.Equal(t, staticAssetsCacheControl, w.Header().Get("Cache-Control"))
				}
			}
		})
	}
}
