//go:build embed || unit

package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// AC-002 (partial): public documentation paths stay separate from the existing SPA alias.
// AC-003 (partial): direct article routes and documentation 404s; client navigation is manual evidence.
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

// AC-005 (partial): every protected script/preload nonce matches the response CSP.
// Heading anchors and actual clipboard content require browser evidence.
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

func writeDocumentationFiles(t *testing.T, directory string, files map[string]string) {
	t.Helper()
	for filename, content := range files {
		filename = filepath.Join(directory, filepath.FromSlash(filename))
		require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0o755))
		require.NoError(t, os.WriteFile(filename, []byte(content), 0o644))
	}
}

func mountedDocumentationFiles() map[string]string {
	return map[string]string{
		"index.html":             `<html><head><script>window.mounted = true;</script></head><body>mounted-docs-home</body></html>`,
		"quick-start.html":       `<html><head><script type="module" src="/docs/assets/app.AbCd1234.js"></script></head><body>mounted-quick-start</body></html>`,
		"nested/index.html":      "mounted-nested-page",
		"404.html":               `<html><head><script>window.mounted404 = true;</script></head><body>mounted-docs-not-found</body></html>`,
		"assets/app.AbCd1234.js": "export const mounted = true;",
		"assets/unhashed.js":     "mounted-unhashed",
		"logo.svg":               `<svg xmlns="http://www.w3.org/2000/svg"><title>mounted-logo</title></svg>`,
	}
}

// AC-008 (partial): mounted pages/assets form one complete site, with no embedded-file mixing.
// Production blue/green mount isolation requires separate deployment evidence.
func TestMountedDocumentationSelectsEntireSite(t *testing.T) {
	directory := t.TempDir()
	writeDocumentationFiles(t, directory, mountedDocumentationFiles())
	t.Setenv("DOCUMENTATION_DIR", directory)
	router := documentationTestRouter(documentationTestFS())
	for _, tc := range []struct {
		path   string
		status int
		body   string
		cache  string
	}{
		{"/docs/", http.StatusOK, "mounted-docs-home", "no-store"},
		{"/docs/quick-start", http.StatusOK, "mounted-quick-start", "no-store"},
		{"/docs/quick-start.html", http.StatusOK, "mounted-quick-start", "no-store"},
		{"/docs/nested/", http.StatusOK, "mounted-nested-page", "no-store"},
		{"/docs/assets/app.AbCd1234.js", http.StatusOK, "export const mounted", staticAssetsCacheControl},
		{"/docs/assets/unhashed.js", http.StatusOK, "mounted-unhashed", ""},
		{"/docs/logo.svg", http.StatusOK, "mounted-logo", ""},
		// Embedded copies exist for both requests. They must not mix with the mounted site.
		{"/docs/guide", http.StatusNotFound, "mounted-docs-not-found", "no-store"},
		{"/docs/assets/page.md.AbCd1234.lean.js", http.StatusNotFound, "mounted-docs-not-found", "no-store"},
		{"/docs/unknown", http.StatusNotFound, "mounted-docs-not-found", "no-store"},
		{"/docs/batch-image", http.StatusOK, "main-spa", ""},
		{"/docs/batch-image/", http.StatusOK, "main-spa", ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			assert.Equal(t, tc.status, w.Code)
			assert.Contains(t, w.Body.String(), tc.body)
			assert.Equal(t, tc.cache, w.Header().Get("Cache-Control"))
		})
	}
}

// AC-008: an absent or incomplete mounted site falls back to the embedded documentation.
func TestMountedDocumentationFallbackRequiresIndex(t *testing.T) {
	for _, state := range []string{"unset", "missing-directory", "missing-index", "index-is-directory"} {
		t.Run(state, func(t *testing.T) {
			directory := t.TempDir()
			writeDocumentationFiles(t, directory, map[string]string{"quick-start.html": "partial-mounted-page"})
			switch state {
			case "unset":
				directory = ""
			case "missing-directory":
				directory = filepath.Join(directory, "missing")
			case "index-is-directory":
				require.NoError(t, os.Mkdir(filepath.Join(directory, "index.html"), 0o755))
			}
			t.Setenv("DOCUMENTATION_DIR", directory)
			router := documentationTestRouter(documentationTestFS())
			for _, path := range []string{"/docs/", "/docs/quick-start"} {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Contains(t, w.Body.String(), "docs-")
				assert.NotContains(t, w.Body.String(), "mounted")
			}
		})
	}
}

// AC-008: mounted document updates and index removal are observed on subsequent requests.
func TestMountedDocumentationUpdatesWithoutRestart(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("DOCUMENTATION_DIR", directory)
	router := documentationTestRouter(documentationTestFS())
	getPage := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/docs/quick-start", nil))
		return w
	}
	assert.Contains(t, getPage().Body.String(), "docs-quick-start")
	writeDocumentationFiles(t, directory, mountedDocumentationFiles())
	assert.Contains(t, getPage().Body.String(), "mounted-quick-start")
	writeDocumentationFiles(t, directory, map[string]string{"quick-start.html": "mounted-updated-page"})
	assert.Contains(t, getPage().Body.String(), "mounted-updated-page")
	require.NoError(t, os.Remove(filepath.Join(directory, "quick-start.html")))
	w := getPage()
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "mounted-docs-not-found")
	require.NoError(t, os.Remove(filepath.Join(directory, "index.html")))
	assert.Contains(t, getPage().Body.String(), "docs-quick-start")
}

// AC-003, AC-008 (partial): a mounted site's missing page cannot use another site's 404.
func TestMountedDocumentationMissing404DoesNotUseEmbedded404(t *testing.T) {
	directory := t.TempDir()
	writeDocumentationFiles(t, directory, map[string]string{"index.html": "mounted-home"})
	t.Setenv("DOCUMENTATION_DIR", directory)
	router := documentationTestRouter(documentationTestFS())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/docs/unknown", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "Documentation not found", w.Body.String())
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

// AC-002, AC-005, AC-008 (partial): mounted documentation preserves route boundaries and nonce handling.
func TestMountedDocumentationPreservesHTTPAndCSP(t *testing.T) {
	directory := t.TempDir()
	writeDocumentationFiles(t, directory, mountedDocumentationFiles())
	t.Setenv("DOCUMENTATION_DIR", directory)
	router := documentationTestRouter(documentationTestFS())
	var previousPolicy string
	for range 2 {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/docs/", nil)
		req.Header.Set("If-None-Match", `"stale"`)
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "mounted-docs-home")
		policy := w.Header().Get("Content-Security-Policy")
		_, rest, found := strings.Cut(policy, "'nonce-")
		require.True(t, found)
		nonce, _, found := strings.Cut(rest, "'")
		require.True(t, found)
		assert.Contains(t, w.Body.String(), `nonce="`+nonce+`"`)
		assert.Contains(t, w.Body.String(), `property="csp-nonce" nonce="`+nonce+`"`)
		assert.NotEqual(t, previousPolicy, policy)
		previousPolicy = policy
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodHead, "/docs/quick-start", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
	assert.NotEmpty(t, w.Header().Get("Content-Length"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/docs/quick-start", nil))
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Equal(t, "GET, HEAD", w.Header().Get("Allow"))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/docs?from=nav", nil))
	assert.Equal(t, http.StatusPermanentRedirect, w.Code)
	assert.Equal(t, "/docs/?from=nav", w.Header().Get("Location"))
	for _, path := range []string{"/docs/../index.html", "/docs/%2e%2e/index.html"} {
		w = httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "mounted-docs-not-found")
	}
}

// AC-002, AC-008 (partial): the mounted public root cannot expose files outside its directory.
func TestMountedDocumentationCannotEscapeDirectoryThroughSymlinks(t *testing.T) {
	directory := t.TempDir()
	writeDocumentationFiles(t, directory, mountedDocumentationFiles())
	outside := filepath.Join(t.TempDir(), "outside.html")
	require.NoError(t, os.WriteFile(outside, []byte("private-file-outside-documentation"), 0o644))
	for _, filename := range []string{"escape.html", "assets/escape.js"} {
		require.NoError(t, os.Symlink(outside, filepath.Join(directory, filepath.FromSlash(filename))))
	}
	t.Setenv("DOCUMENTATION_DIR", directory)
	router := documentationTestRouter(documentationTestFS())
	for _, path := range []string{"/docs/escape", "/docs/assets/escape.js"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "mounted-docs-not-found")
		assert.NotContains(t, w.Body.String(), "private-file-outside-documentation")
	}
}
