package fasthttp

import (
	"bytes"
	"io"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// setupTestDir creates a temporary directory with test files and returns the path.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"index.html":        "<h1>Hello</h1>",
		"js/app.js":         "console.log('hello')",
		"css/style.css":     "body { color: red; }",
		"sub/deep/file.txt": "deep file",
		"spa/index.html":    "<div id=\"root\"></div>",
	}

	for name, content := range files {
		fp := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestStatic_ServeFile(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/static", dir)

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/static/js/app.js")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", ctx.Response.StatusCode())
	}
	body := string(ctx.Response.Body())
	if !strings.Contains(body, "console.log") {
		t.Errorf("body = %q, want to contain 'console.log'", body)
	}
	ct := string(ctx.Response.Header.ContentType())
	if !strings.Contains(ct, "text/javascript") {
		t.Errorf("content-type = %q, want text/javascript", ct)
	}
}

func TestStatic_ServeIndex(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/static", dir)

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/static/")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", ctx.Response.StatusCode())
	}
	body := string(ctx.Response.Body())
	if !strings.Contains(body, "<h1>Hello</h1>") {
		t.Errorf("body = %q, want to contain '<h1>Hello</h1>'", body)
	}
}

func TestStatic_NotFound(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/static", dir)

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/static/does-not-exist.txt")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusNotFound {
		t.Fatalf("status = %d, want 404", ctx.Response.StatusCode())
	}
}

func TestStatic_SPAFallback(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/", dir, StaticConfig{NotFound: "spa/index.html"})

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/some/spa/route")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", ctx.Response.StatusCode())
	}
	body := string(ctx.Response.Body())
	if !strings.Contains(body, "<div id=\"root\">") {
		t.Errorf("body = %q, want to contain SPA root", body)
	}
}

func TestStatic_DirectoryTraversalPrevention(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/static", dir)

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/static/../../../etc/passwd")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	// fasthttp normalizes Path() before routing removes ".." segments,
	// so the catch-all route doesn't match and returns 404.
	// This is correct security behavior — the traversal attempt is blocked
	// because it doesn't reach any handler.
	if ctx.Response.StatusCode() != fasthttp.StatusNotFound {
		t.Fatalf("status = %d, want 404 (normalized path doesn't match static route)", ctx.Response.StatusCode())
	}
}

func TestStatic_CacheControlMaxAge(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/static", dir, StaticConfig{MaxAge: 3600})

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/static/js/app.js")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", ctx.Response.StatusCode())
	}
	cc := string(ctx.Response.Header.Peek("Cache-Control"))
	if cc != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q, want public, max-age=3600", cc)
	}
}

func TestStatic_ETag(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/static", dir)

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/static/js/app.js")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", ctx.Response.StatusCode())
	}
	etag := string(ctx.Response.Header.Peek("ETag"))
	if etag == "" {
		t.Fatal("ETag is empty")
	}

	// Second request with If-None-Match should return 304
	ctx2 := new(fasthttp.RequestCtx)
	ctx2.Request.SetRequestURI("/static/js/app.js")
	ctx2.Request.Header.SetMethod("GET")
	ctx2.Request.Header.Set("If-None-Match", etag)

	h(ctx2)

	if ctx2.Response.StatusCode() != fasthttp.StatusNotModified {
		t.Fatalf("status = %d, want 304", ctx2.Response.StatusCode())
	}
}

func TestStatic_ContentTypeByExtension(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/static", dir)

	h := app.Handler()
	tests := []struct {
		uri      string
		wantType string
	}{
		{"/static/js/app.js", "text/javascript"},
		{"/static/css/style.css", "text/css"},
		{"/static/index.html", "text/html"},
	}

	for _, tt := range tests {
		ctx := new(fasthttp.RequestCtx)
		ctx.Request.SetRequestURI(tt.uri)
		ctx.Request.Header.SetMethod("GET")
		h(ctx)
		ct := string(ctx.Response.Header.ContentType())
		if !strings.Contains(ct, tt.wantType) {
			t.Errorf("%s: content-type = %q, want to contain %q", tt.uri, ct, tt.wantType)
		}
	}
}

func TestStatic_RootPrefix(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/", dir)

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/js/app.js")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", ctx.Response.StatusCode())
	}
	body := string(ctx.Response.Body())
	if !strings.Contains(body, "console.log") {
		t.Errorf("body = %q, want to contain 'console.log'", body)
	}
}

func TestStatic_NestedSubdirectory(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/static", dir)

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/static/sub/deep/file.txt")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", ctx.Response.StatusCode())
	}
	body := string(ctx.Response.Body())
	if !strings.Contains(body, "deep file") {
		t.Errorf("body = %q, want to contain 'deep file'", body)
	}
}

func TestStatic_ServeFileOnFastCtx(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Get("/", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		return fc.SendFile(filepath.Join(dir, "index.html"))
	})

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod("GET")

	h(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", ctx.Response.StatusCode())
	}
	body := string(ctx.Response.Body())
	if !strings.Contains(body, "<h1>Hello</h1>") {
		t.Errorf("body = %q, want to contain '<h1>Hello</h1>'", body)
	}
}

// --- Form & File Upload Tests ---

func TestFastCtx_FormValue(t *testing.T) {
	app := New()
	var name string
	app.Post("/submit", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		name = fc.FormValue("name")
		return nil
	})

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/submit")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentType("application/x-www-form-urlencoded")
	ctx.Request.SetBodyString("name=John&email=john@example.com")

	h(ctx)

	if name != "John" {
		t.Errorf("FormValue = %q, want John", name)
	}
}

func TestFastCtx_FormFile(t *testing.T) {
	app := New()
	var fileName string
	var fileData []byte
	app.Post("/upload", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		fh, err := fc.FormFile("file")
		if err != nil {
			return err
		}
		fileName = fh.Filename
		fileData, err = fh.ReadAll()
		if err != nil {
			return err
		}
		return nil
	})

	// Build multipart form body
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fileWriter, err := w.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(fileWriter, "file content"); err != nil {
		t.Fatal(err)
	}
	w.Close()

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/upload")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentType(w.FormDataContentType())
	ctx.Request.SetBody(b.Bytes())

	h(ctx)

	if fileName != "test.txt" {
		t.Errorf("FormFile filename = %q, want test.txt", fileName)
	}
	if string(fileData) != "file content" {
		t.Errorf("FormFile data = %q, want 'file content'", fileData)
	}
}

func TestFastCtx_FormFiles(t *testing.T) {
	app := New()
	var files []*FileHeader
	app.Post("/upload", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		var err error
		files, err = fc.FormFiles("files")
		return err
	})

	// Build multipart form body with multiple files
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw1, _ := w.CreateFormFile("files", "a.txt")
	io.WriteString(fw1, "file A")
	fw2, _ := w.CreateFormFile("files", "b.txt")
	io.WriteString(fw2, "file B")
	w.Close()

	h := app.Handler()
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/upload")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentType(w.FormDataContentType())
	ctx.Request.SetBody(b.Bytes())

	h(ctx)

	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0].Filename != "a.txt" || string(files[0].content) != "file A" {
		t.Errorf("first file: name=%q data=%q", files[0].Filename, files[0].content)
	}
	if files[1].Filename != "b.txt" || string(files[1].content) != "file B" {
		t.Errorf("second file: name=%q data=%q", files[1].Filename, files[1].content)
	}
}

// --- Integration Test: Server ---

func TestStaticIntegration(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/static", dir)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		app.server = &fasthttp.Server{Handler: app.Handler()}
		_ = app.server.Serve(ln)
	}()

	addr := ln.Addr().String()
	statusCode, resp, err := fasthttp.Get(nil, "http://"+addr+"/static/js/app.js")
	if err != nil {
		t.Fatal(err)
	}
	if statusCode != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", statusCode)
	}
	if !strings.Contains(string(resp), "console.log") {
		t.Errorf("body = %q, want to contain 'console.log'", resp)
	}
}

func TestStaticIntegration_404(t *testing.T) {
	dir := setupTestDir(t)
	app := New()
	app.Static("/static", dir)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		app.server = &fasthttp.Server{Handler: app.Handler()}
		_ = app.server.Serve(ln)
	}()

	addr := ln.Addr().String()
	statusCode, _, err := fasthttp.Get(nil, "http://"+addr+"/static/does-not-exist.txt")
	if err != nil {
		t.Fatal(err)
	}
	if statusCode != fasthttp.StatusNotFound {
		t.Errorf("status = %d, want 404", statusCode)
	}
}
