package fasthttp

import (
	"bytes"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func TestWrite_SetBodyRaw(t *testing.T) {
	app := New(domain.Config{DisableRequestID: true, Security: domain.SecurityConfig{DisableHeaders: true}})
	body := []byte(`{"ok":true}`)
	app.Get("/", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		return fc.Write(200, body)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod("GET")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("status = %d", ctx.Response.StatusCode())
	}
	if !bytes.Equal(ctx.Response.Body(), body) {
		t.Fatalf("body = %q", ctx.Response.Body())
	}
	if ct := string(ctx.Response.Header.ContentType()); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	// SetBodyRaw: response body shares the slice (Fiber Send semantics).
	if &ctx.Response.Body()[0] != &body[0] {
		t.Fatal("Write should use SetBodyRaw (same backing array)")
	}
}

func TestBytes_SetBodyRaw(t *testing.T) {
	app := New(domain.Config{DisableRequestID: true, Security: domain.SecurityConfig{DisableHeaders: true}})
	body := []byte("raw-bytes")
	app.Get("/", func(c *domain.Ctx) error {
		return c.AdapterData.(*FastCtx).Bytes(200, body, "application/octet-stream")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod("GET")
	app.Handler()(ctx)

	if &ctx.Response.Body()[0] != &body[0] {
		t.Fatal("Bytes should use SetBodyRaw")
	}
}

func TestInlineRecover_DefaultCatchesPanic(t *testing.T) {
	app := New(domain.Config{DisableRequestID: true, Security: domain.SecurityConfig{DisableHeaders: true}})
	app.Get("/panic", func(c *domain.Ctx) error {
		panic("boom")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/panic")
	ctx.Request.Header.SetMethod("GET")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 500 {
		t.Fatalf("status = %d, want 500", ctx.Response.StatusCode())
	}
}

func TestInlineRecover_DisabledPropagates(t *testing.T) {
	app := New(domain.Config{
		DisableRequestID:     true,
		DisableInlineRecover: true,
		Security:             domain.SecurityConfig{DisableHeaders: true},
	})
	app.Get("/panic", func(c *domain.Ctx) error {
		panic("boom")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/panic")
	ctx.Request.Header.SetMethod("GET")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to propagate when DisableInlineRecover")
		}
	}()
	app.Handler()(ctx)
}

func TestHandlerHot_OK(t *testing.T) {
	app := New(domain.Config{
		DisableRequestID:     true,
		DisableInlineRecover: true,
		Security:             domain.SecurityConfig{DisableHeaders: true},
	})
	body := []byte(`{"hot":true}`)
	app.Get("/", func(c *domain.Ctx) error {
		return c.AdapterData.(*FastCtx).Write(200, body)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod("GET")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 || !bytes.Equal(ctx.Response.Body(), body) {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}
