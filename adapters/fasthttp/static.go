package fasthttp

import (
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// StaticConfig configures static file server behavior.
type StaticConfig struct {
	// Index is the index file name (default: "index.html").
	Index string
	// NotFound is the SPA fallback file (e.g. "index.html").
	NotFound string
	// SinglePageApp, if true, uses Index (or "index.html") as 404 fallback (SPA).
	SinglePageApp bool
	// MaxAge is seconds for Cache-Control max-age (0 = no cache).
	MaxAge int
	// Compress enables gzip compression (default: true, depends on CompressMiddleware).
	Compress bool
}

func (c *StaticConfig) index() string {
	if c.Index != "" {
		return c.Index
	}
	return "index.html"
}

func (c *StaticConfig) notFoundFile() string {
	if c.NotFound != "" {
		return c.NotFound
	}
	if c.SinglePageApp {
		return c.index()
	}
	return ""
}

// Static serves static files from a directory.
// Examples:
//
//	app.Static("/", "./public")           // Serve ./public at root
//	app.Static("/assets", "./assets")     // /assets/file.js → ./assets/file.js
//	app.Static("/", "./public", StaticConfig{SinglePageApp: true}) // SPA mode
func (a *App) Static(prefix, root string, cfg ...StaticConfig) *Route {
	config := StaticConfig{}
	if len(cfg) > 0 {
		config = cfg[0]
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	// resolvedRoot follows any symlink on the root directory itself, so
	// the per-request symlink check below compares against the real
	// filesystem location rather than a symlinked alias of it.
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		resolvedRoot = absRoot
	}

	spaFallback := config.notFoundFile()

	handler := func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		fctx := fc.FasthttpCtx()

		// Extract file path from request URL
		filePath := string(fctx.Path())

		// Remove prefix to get relative path
		if prefix != "/" {
			filePath = strings.TrimPrefix(filePath, prefix)
		}

		filePath = path.Clean(filePath)

		// Prevent directory traversal: check multiple encoding variants
		// fasthttp normalizes the path, but we also check the raw URI
		// for encoded traversal attempts (%2e%2e, %252e%252e, etc.)
		rawURI := string(fctx.RequestURI())
		decodedURI, _ := url.QueryUnescape(rawURI)
		if strings.Contains(rawURI, "..") ||
			strings.Contains(decodedURI, "..") ||
			strings.Contains(strings.ToLower(rawURI), "%2e") ||
			strings.Contains(strings.ToLower(decodedURI), "%2e") ||
			strings.Contains(rawURI, "%252e") ||
			strings.Contains(filePath, "..") {
			fctx.SetStatusCode(fasthttp.StatusForbidden)
			fctx.SetBodyString("Forbidden")
			return nil
		}

		// Additional check: resolve the full path and ensure it's within root
		fullPath := filepath.Join(absRoot, filePath)
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			fctx.SetStatusCode(fasthttp.StatusForbidden)
			fctx.SetBodyString("Forbidden")
			return nil
		}
		rel, err := filepath.Rel(absRoot, absPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			fctx.SetStatusCode(fasthttp.StatusForbidden)
			fctx.SetBodyString("Forbidden")
			return nil
		}

		// Resolve symlinks before serving: the checks above only validate
		// the requested *path string* against root, but a symlink placed
		// inside root (e.g. "public/evil -> /etc/passwd", or planted by
		// an attacker who can write one file into an otherwise-safe
		// upload/static directory) can point anywhere on disk regardless
		// of how clean the request path looks. Re-validate the fully
		// resolved real path against resolvedRoot before serving.
		if resolved, err := filepath.EvalSymlinks(fullPath); err == nil {
			relResolved, err := filepath.Rel(resolvedRoot, resolved)
			if err != nil || relResolved == ".." || strings.HasPrefix(relResolved, ".."+string(filepath.Separator)) {
				fctx.SetStatusCode(fasthttp.StatusForbidden)
				fctx.SetBodyString("Forbidden")
				return nil
			}
			fullPath = resolved
		}

		// Try to serve the file
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			return serveFile(fc, fullPath, config.MaxAge)
		}

		// Try index file for directories
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			indexPath := filepath.Join(fullPath, config.index())
			if fi, e := os.Stat(indexPath); e == nil && !fi.IsDir() {
				return serveFile(fc, indexPath, config.MaxAge)
			}
		}

		// SPA fallback
		if spaFallback != "" {
			notFoundPath := spaFallback
			if !filepath.IsAbs(notFoundPath) {
				notFoundPath = filepath.Join(absRoot, notFoundPath)
			}
			if fi, e := os.Stat(notFoundPath); e == nil && !fi.IsDir() {
				return serveFile(fc, notFoundPath, config.MaxAge)
			}
		}

		// 404
		fctx.SetStatusCode(fasthttp.StatusNotFound)
		fctx.SetBodyString("Not Found")
		return nil
	}

	return a.Get(prefix+"/*filepath", handler)
}

func serveFile(fc *FastCtx, fullPath string, maxAge int) error {
	fctx := fc.FasthttpCtx()

	file, err := os.Open(fullPath)
	if err != nil {
		fctx.SetStatusCode(fasthttp.StatusNotFound)
		return nil
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil || stat.IsDir() {
		fctx.SetStatusCode(fasthttp.StatusNotFound)
		return nil
	}

	// Content-Type
	ext := filepath.Ext(fullPath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	fctx.Response.Header.SetContentType(mimeType)

	// Cache-Control
	if maxAge > 0 {
		fctx.Response.Header.Set("Cache-Control", "public, max-age="+itoa(maxAge))
	}

	// Content-Length
	fctx.Response.Header.SetContentLength(int(stat.Size()))

	// ETag (hash of mtime + size to avoid timing information disclosure)
	h := sha256.Sum256([]byte(fmt.Sprintf("%d-%d", stat.ModTime().UnixNano(), stat.Size())))
	etag := "\"" + fmt.Sprintf("%x", h) + "\""
	fctx.Response.Header.Set("ETag", etag)

	// Conditional request (304 Not Modified)
	if match := string(fctx.Request.Header.Peek("If-None-Match")); match == etag {
		fctx.SetStatusCode(fasthttp.StatusNotModified)
		return nil
	}

	// Send file using fasthttp's built-in SendFile
	fctx.SendFile(fullPath)
	return nil
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// SendFile sent a file from the local filesystem as the response body.
// The method is available on the context for handler usage.
func (c *FastCtx) SendFile(path string) error {
	fctx := c.FasthttpCtx()
	fctx.SendFile(path)
	return nil
}

// ensure interfaces are satisfied
var _ io.Reader = nil
