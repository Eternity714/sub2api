//go:build embed || unit

package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func documentationTestFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                           {Data: []byte("main-spa")},
		"docs/index.html":                      {Data: []byte(`<html><head><script>window.docs = true;</script></head><body>docs-home</body></html>`)},
		"docs/quick-start.html":                {Data: []byte(`<html><head><script type="module" src="/docs/assets/app.AbCd1234.js"></script></head><body>docs-quick-start</body></html>`)},
		"docs/guide/index.html":                {Data: []byte("docs-guide")},
		"docs/404.html":                        {Data: []byte(`<html><head><script>window.docs404 = true;</script></head><body>docs-not-found</body></html>`)},
		"docs/assets/app.AbCd1234.js":          {Data: []byte("export const docs = true;")},
		"docs/assets/page.md.AbCd1234.lean.js": {Data: []byte("export const page = true;")},
		"docs/assets/style-AbCd1234.css":       {Data: []byte("body { color: blue; }")},
		"docs/assets/unhashed.js":              {Data: []byte("unhashed")},
		"docs/logo.svg":                        {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)},
	}
}

func documentationTestRouter(fsys fs.FS) *gin.Engine {
	router := gin.New()
	router.Use(middleware.SecurityHeaders(config.CSPConfig{Enabled: true}, nil))
	router.Use(func(c *gin.Context) {
		if !serveDocumentation(c, fsys) {
			c.String(http.StatusOK, "main-spa")
		}
	})
	return router
}

func TestDocumentationRoutes(t *testing.T) {
	router := documentationTestRouter(documentationTestFS())
	for _, tc := range []struct {
		path string
		body string
	}{
		{"/docs/", "docs-home"},
		{"/docs/index.html", "docs-home"},
		{"/docs/quick-start", "docs-quick-start"},
		{"/docs/quick-start.html", "docs-quick-start"},
		{"/docs/guide", "docs-guide"},
		{"/docs/guide/", "docs-guide"},
		{"/docs/batch-image", "main-spa"},
		{"/docs/batch-image/", "main-spa"},
		{"/docs-other", "main-spa"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), tc.body)
			if tc.body != "main-spa" {
				assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			}
		})
	}

	t.Run("canonical_root_preserves_query", func(t *testing.T) {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/docs?from=nav", nil))
		assert.Equal(t, http.StatusPermanentRedirect, w.Code)
		assert.Equal(t, "/docs/?from=nav", w.Header().Get("Location"))
	})

	t.Run("missing_page_stays_in_documentation", func(t *testing.T) {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/docs/unknown-page", nil))
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "docs-not-found")
		assert.NotContains(t, w.Body.String(), "main-spa")
		assert.Contains(t, w.Body.String(), `nonce="`)
	})

	t.Run("unbuilt_documentation_returns_404", func(t *testing.T) {
		unbuiltRouter := documentationTestRouter(fstest.MapFS{"index.html": {Data: []byte("main-spa")}})
		for _, path := range []string{"/docs/", "/docs/quick-start", "/docs/assets/missing.js"} {
			w := httptest.NewRecorder()
			unbuiltRouter.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			assert.Equal(t, http.StatusNotFound, w.Code, path)
			assert.NotContains(t, w.Body.String(), "main-spa", path)
		}
	})

	t.Run("invalid_paths_do_not_escape_documentation", func(t *testing.T) {
		for _, path := range []string{"/docs/../index.html", "/docs//../index.html", "/docs/%2e%2e/index.html"} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			assert.Equal(t, http.StatusNotFound, w.Code, path)
			assert.NotContains(t, w.Body.String(), "main-spa", path)
		}
	})

	t.Run("read_only_methods", func(t *testing.T) {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/docs/quick-start", nil))
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		assert.Equal(t, "GET, HEAD", w.Header().Get("Allow"))
	})

	t.Run("head_has_headers_without_body", func(t *testing.T) {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodHead, "/docs/quick-start", nil))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Body.String())
		assert.NotEmpty(t, w.Header().Get("Content-Length"))
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	})
}

func TestDocumentationAssets(t *testing.T) {
	router := documentationTestRouter(documentationTestFS())
	for _, tc := range []struct {
		path  string
		cache string
		mime  string
	}{
		{"/docs/assets/app.AbCd1234.js", staticAssetsCacheControl, "javascript"},
		{"/docs/assets/page.md.AbCd1234.lean.js", staticAssetsCacheControl, "javascript"},
		{"/docs/assets/style-AbCd1234.css", staticAssetsCacheControl, "text/css"},
		{"/docs/assets/unhashed.js", "", "javascript"},
		{"/docs/logo.svg", "", "image/svg+xml"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tc.cache, w.Header().Get("Cache-Control"))
			assert.Contains(t, w.Header().Get("Content-Type"), tc.mime)
		})
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/docs/assets/missing.js", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotEqual(t, staticAssetsCacheControl, w.Header().Get("Cache-Control"))
}

func TestDocumentationCSPNonce(t *testing.T) {
	content := `<!doctype html><html><head><link rel="modulepreload" href="/docs/assets/app.AbCd1234.js"><link rel="preload" as="script" href="/docs/assets/chunk.AbCd1234.js"><link rel="stylesheet" href="/docs/assets/style.css"><script nonce="stale" type="module" src="/docs/assets/app.AbCd1234.js"></script><script>window.inline = "<script>literal";</script></head><body>docs-csp</body></html>`
	fsy := documentationTestFS()
	fsy["docs/index.html"] = &fstest.MapFile{Data: []byte(content)}
	router := documentationTestRouter(fsy)
	var previousNonce string
	for range 2 {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/docs/", nil)
		req.Header.Set("If-None-Match", `"old-page"`)
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		policy := w.Header().Get("Content-Security-Policy")
		_, rest, ok := strings.Cut(policy, "'nonce-")
		require.True(t, ok)
		nonce, _, ok := strings.Cut(rest, "'")
		require.True(t, ok)
		require.NotEmpty(t, nonce)
		assert.NotEqual(t, previousNonce, nonce)
		previousNonce = nonce
		for _, directive := range strings.Split(policy, ";") {
			if strings.HasPrefix(strings.TrimSpace(directive), "script-src ") {
				assert.NotContains(t, directive, "'unsafe-inline'", "script policy remains nonce based")
			}
		}
		assert.NotContains(t, w.Body.String(), `nonce="stale"`)
		assert.Contains(t, w.Body.String(), `window.inline = "<script>literal";`)
		assert.Contains(t, w.Body.String(), `property="csp-nonce" nonce="`+nonce+`"`)

		var protectedTags int
		z := html.NewTokenizer(strings.NewReader(w.Body.String()))
		for z.Next() != html.ErrorToken {
			token := z.Token()
			if token.Type != html.StartTagToken {
				continue
			}
			attrs := make(map[string]string)
			for _, attr := range token.Attr {
				attrs[attr.Key] = attr.Val
			}
			if token.Data == "script" || (token.Data == "link" && (attrs["rel"] == "modulepreload" || attrs["as"] == "script")) {
				assert.Equal(t, nonce, attrs["nonce"])
				protectedTags++
			}
		}
		assert.Equal(t, 4, protectedTags)
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	}
}
