package fasthttp

import (
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func TestCORSAllowAll(t *testing.T) {
	app := New()
	app.Use(CORSMiddleware(domain.CORSConfig{
		AllowedOrigins: []string{"*"},
	}))
	app.Get("/test", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	ctx.Request.Header.Set("Origin", "https://example.com")
	app.Handler()(ctx)

	if string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")) != "*" {
		t.Errorf("Allow-Origin = %q, want *", ctx.Response.Header.Peek("Access-Control-Allow-Origin"))
	}
}

func TestCORSWithOrigin(t *testing.T) {
	app := New()
	app.Use(CORSMiddleware(domain.CORSConfig{
		AllowedOrigins: []string{"https://app.com"},
	}))
	app.Get("/test", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	ctx.Request.Header.Set("Origin", "https://app.com")
	app.Handler()(ctx)

	if string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")) != "https://app.com" {
		t.Errorf("Allow-Origin = %q, want https://app.com", ctx.Response.Header.Peek("Access-Control-Allow-Origin"))
	}
}

func TestCORSBlockedOrigin(t *testing.T) {
	app := New()
	app.Use(CORSMiddleware(domain.CORSConfig{
		AllowedOrigins: []string{"https://app.com"},
	}))
	app.Get("/test", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	ctx.Request.Header.Set("Origin", "https://evil.com")
	app.Handler()(ctx)

	// No Origin header should be set for blocked origins
	if len(ctx.Response.Header.Peek("Access-Control-Allow-Origin")) > 0 {
		t.Errorf("Allow-Origin = %q, want empty for blocked origin", ctx.Response.Header.Peek("Access-Control-Allow-Origin"))
	}
}

func TestCORSPreflight(t *testing.T) {
	app := New()
	app.Use(CORSMiddleware(domain.CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
	}))
	app.Get("/test", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	ctx.Request.Header.SetMethod("OPTIONS")
	ctx.Request.Header.Set("Origin", "https://example.com")
	ctx.Request.Header.Set("Access-Control-Request-Method", "POST")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 204 {
		t.Errorf("StatusCode = %d, want 204 for preflight", ctx.Response.StatusCode())
	}
	if string(ctx.Response.Header.Peek("Access-Control-Allow-Methods")) == "" {
		t.Error("Allow-Methods header not set")
	}
}

func TestCORSNoOrigin(t *testing.T) {
	app := New()
	app.Use(CORSMiddleware())
	app.Get("/test", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	// No origin header should be set when request has no Origin
	if len(ctx.Response.Header.Peek("Access-Control-Allow-Origin")) > 0 {
		t.Errorf("Allow-Origin = %q, want empty when no Origin in request", ctx.Response.Header.Peek("Access-Control-Allow-Origin"))
	}
}

func TestCORSWithCredentials(t *testing.T) {
	app := New()
	app.Use(CORSMiddleware(domain.CORSConfig{
		AllowedOrigins:   []string{"https://app.com"},
		AllowCredentials: true,
	}))
	app.Get("/test", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	ctx.Request.Header.Set("Origin", "https://app.com")
	app.Handler()(ctx)

	if string(ctx.Response.Header.Peek("Access-Control-Allow-Credentials")) != "true" {
		t.Errorf("Allow-Credentials = %q, want true", ctx.Response.Header.Peek("Access-Control-Allow-Credentials"))
	}
}

func TestRecoverPanic(t *testing.T) {
	app := New()
	app.Use(RecoverMiddleware())
	app.Get("/panic", func(c *domain.Ctx) error {
		panic("test panic")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/panic")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 500 {
		t.Errorf("StatusCode = %d, want 500 after panic", ctx.Response.StatusCode())
	}
}

// TestRecoverInline_GatesStackTraceByDebugFlag covers V-33: stack trace
// exposure on a recovered panic must be controlled by an explicit
// Config.Debug flag, not tied to Env (which also gates CORS/HSTS and
// could be EnvDevelopment on a reachable staging deployment for other,
// unrelated reasons). The inline defer/recover in the request handler —
// not RecoverMiddleware — is what actually catches panics (see
// recoverBody's doc comment), so that is what must be gated here.
func TestRecoverInline_GatesStackTraceByDebugFlag(t *testing.T) {
	appDebugOff := New(domain.Config{Debug: false})
	appDebugOff.Get("/panic", func(c *domain.Ctx) error { panic("boom") })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/panic")
	appDebugOff.Handler()(ctx)
	if strings.Contains(string(ctx.Response.Body()), "stack") {
		t.Fatalf("expected no stack trace in body with debug=false, got %q", ctx.Response.Body())
	}

	appDebugOn := New(domain.Config{Debug: true})
	appDebugOn.Get("/panic", func(c *domain.Ctx) error { panic("boom") })

	ctx2 := &fasthttp.RequestCtx{}
	ctx2.Request.SetRequestURI("/panic")
	appDebugOn.Handler()(ctx2)
	if !strings.Contains(string(ctx2.Response.Body()), "stack") {
		t.Fatalf("expected stack trace body with debug=true, got %q", ctx2.Response.Body())
	}
}

func TestRecoverNoPanic(t *testing.T) {
	app := New()
	app.Use(RecoverMiddleware())
	app.Get("/ok", func(c *domain.Ctx) error {
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/ok")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", ctx.Response.StatusCode())
	}
}

func TestTimeoutWithConfig(t *testing.T) {
	app := New(domain.Config{
		RequestTimeout: 100 * time.Millisecond,
	})
	app.Get("/fast", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/fast")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", ctx.Response.StatusCode())
	}
}

func TestCompressPostMiddleware(t *testing.T) {
	app := New()
	// Compress must run as post middleware to have access to response body
	app.UsePost(CompressMiddleware())
	app.Get("/large", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		fc.FasthttpCtx().Response.SetBodyString(string(make([]byte, 2048)))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/large")
	ctx.Request.Header.Set("Accept-Encoding", "gzip")
	app.Handler()(ctx)

	// Should be compressed (content-encoding: gzip)
	if string(ctx.Response.Header.ContentEncoding()) != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", ctx.Response.Header.ContentEncoding())
	}

	// Body should be smaller after compression
	if len(ctx.Response.Body()) >= 2048 {
		t.Errorf("Compressed body too large: %d, want < 2048", len(ctx.Response.Body()))
	}
}

func TestCompressSkipsSmallBody(t *testing.T) {
	app := New()
	app.UsePost(CompressMiddleware())
	app.Get("/small", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		fc.FasthttpCtx().Response.SetBodyString("small")
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/small")
	ctx.Request.Header.Set("Accept-Encoding", "gzip")
	app.Handler()(ctx)

	// Small bodies should not be compressed
	if len(ctx.Response.Header.ContentEncoding()) > 0 {
		t.Errorf("Content-Encoding = %q, want empty for small body", ctx.Response.Header.ContentEncoding())
	}
}

func TestCompressNoAcceptEncoding(t *testing.T) {
	app := New()
	app.UsePost(CompressMiddleware())
	app.Get("/data", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		fc.FasthttpCtx().Response.SetBodyString(string(make([]byte, 2048)))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/data")
	// No Accept-Encoding header
	app.Handler()(ctx)

	if len(ctx.Response.Header.ContentEncoding()) > 0 {
		t.Errorf("Content-Encoding = %q, want empty when client doesn't accept gzip", ctx.Response.Header.ContentEncoding())
	}
}

func TestNoCORSWithoutHeader(t *testing.T) {
	app := New()
	app.Use(CORSMiddleware())
	app.Get("/test", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	if len(ctx.Response.Header.Peek("Access-Control-Allow-Origin")) > 0 {
		t.Errorf("Allow-Origin = %q, want empty for non-CORS request", ctx.Response.Header.Peek("Access-Control-Allow-Origin"))
	}
}
