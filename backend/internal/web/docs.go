//go:build embed || unit

package web

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

// serveDocumentation owns the public VitePress namespace. Missing pages must
// remain documentation 404s rather than falling through to the authenticated SPA.
func serveDocumentation(c *gin.Context, distFS fs.FS) bool {
	requestPath := c.Request.URL.Path
	if requestPath != "/docs" && !strings.HasPrefix(requestPath, "/docs/") {
		return false
	}
	// Keep the pre-existing SPA alias and its authentication behavior intact.
	if requestPath == "/docs/batch-image" || requestPath == "/docs/batch-image/" {
		return false
	}
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.Header("Allow", "GET, HEAD")
		c.AbortWithStatus(http.StatusMethodNotAllowed)
		return true
	}
	if requestPath == "/docs" {
		location := "/docs/"
		if c.Request.URL.RawQuery != "" {
			location += "?" + c.Request.URL.RawQuery
		}
		c.Redirect(http.StatusPermanentRedirect, location)
		c.Abort()
		return true
	}

	pagePath := strings.TrimSuffix(strings.TrimPrefix(requestPath, "/docs/"), "/")
	if pagePath != "" && !fs.ValidPath(pagePath) {
		serveDocumentationNotFound(c, distFS)
		return true
	}

	var candidates []string
	if pagePath == "" {
		candidates = []string{"docs/index.html"}
	} else if path.Ext(pagePath) == "" {
		// VitePress cleanUrls emits foo.html while exposing /docs/foo.
		candidates = []string{"docs/" + pagePath + ".html", "docs/" + pagePath + "/index.html"}
	} else {
		candidates = []string{"docs/" + pagePath}
	}
	for _, filename := range candidates {
		content, err := fs.ReadFile(distFS, filename)
		if err != nil {
			continue
		}
		if path.Ext(filename) == ".html" {
			serveDocumentationHTML(c, http.StatusOK, content)
		} else {
			if isFingerprintedDocumentationAssetPath(filename) {
				c.Header("Cache-Control", staticAssetsCacheControl)
			}
			http.ServeContent(c.Writer, c.Request, filename, time.Time{}, bytes.NewReader(content))
			c.Abort()
		}
		return true
	}
	serveDocumentationNotFound(c, distFS)
	return true
}

func serveDocumentationNotFound(c *gin.Context, distFS fs.FS) {
	if content, err := fs.ReadFile(distFS, "docs/404.html"); err == nil {
		serveDocumentationHTML(c, http.StatusNotFound, content)
		return
	}
	c.Header("Cache-Control", "no-store")
	if c.Request.Method == http.MethodHead {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.String(http.StatusNotFound, "Documentation not found")
	c.Abort()
}

func serveDocumentationHTML(c *gin.Context, status int, content []byte) {
	content = injectDocumentationNonce(content, middleware.GetNonceFromContext(c))
	// The CSP header and HTML carry a fresh matching nonce for each request.
	// Avoid both cached HTML and conditional 304s with a different CSP nonce.
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Length", strconv.Itoa(len(content)))
	c.Header("Content-Type", "text/html; charset=utf-8")
	if c.Request.Method == http.MethodHead {
		c.Status(status)
	} else {
		c.Data(status, "text/html; charset=utf-8", content)
	}
	c.Abort()
}

// injectDocumentationNonce preserves raw script text while authorizing inline
// bootstrap scripts, module entrypoints and their script preloads under CSP.
func injectDocumentationNonce(content []byte, nonce string) []byte {
	if nonce == "" {
		return content
	}
	var out bytes.Buffer
	out.Grow(len(content))
	z := html.NewTokenizer(bytes.NewReader(content))
	for {
		tokenType := z.Next()
		if tokenType == html.ErrorToken {
			out.Write(z.Raw())
			return out.Bytes()
		}
		if tokenType == html.EndTagToken {
			token := z.Token()
			if token.Data == "head" {
				// Vite reads this nonce when adding preloads during navigation.
				meta := html.Token{Type: html.SelfClosingTagToken, Data: "meta", Attr: []html.Attribute{
					{Key: "property", Val: "csp-nonce"},
					{Key: "nonce", Val: nonce},
				}}
				out.WriteString(meta.String())
			}
			out.Write(z.Raw())
			continue
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			out.Write(z.Raw())
			continue
		}
		token := z.Token()
		protected := token.Data == "script"
		if token.Data == "link" {
			var rel, as string
			for _, attr := range token.Attr {
				switch attr.Key {
				case "rel":
					rel = strings.ToLower(attr.Val)
				case "as":
					as = strings.ToLower(attr.Val)
				}
			}
			for _, value := range strings.Fields(rel) {
				if value == "modulepreload" || (value == "preload" && as == "script") {
					protected = true
				}
			}
		}
		if !protected {
			out.Write(z.Raw())
			continue
		}
		attributes := token.Attr[:0]
		for _, attr := range token.Attr {
			if attr.Key != "nonce" {
				attributes = append(attributes, attr)
			}
		}
		token.Attr = append(attributes, html.Attribute{Key: "nonce", Val: nonce})
		out.WriteString(token.String())
	}
}

func isFingerprintedDocumentationAssetPath(filename string) bool {
	const prefix = "docs/assets/"
	if !strings.HasPrefix(filename, prefix) {
		return false
	}
	assetPath := strings.TrimPrefix(filename, "docs/")
	if isFingerprintedEmbeddedAssetPath(assetPath) {
		return true
	}
	// VitePress uses name.<hash>.js and name.<hash>.lean.js, unlike the
	// main application's name-<hash>.js asset naming.
	stem := strings.TrimSuffix(path.Base(filename), path.Ext(filename))
	stem = strings.TrimSuffix(stem, ".lean")
	_, fingerprint, ok := strings.Cut(stem, ".")
	if !ok {
		return false
	}
	if lastDot := strings.LastIndexByte(fingerprint, '.'); lastDot >= 0 {
		fingerprint = fingerprint[lastDot+1:]
	}
	return isFingerprintedEmbeddedAssetPath("assets/doc-" + fingerprint + path.Ext(filename))
}
