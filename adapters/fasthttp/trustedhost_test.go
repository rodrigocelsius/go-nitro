package fasthttp

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// TestTrustedHostMiddleware_RejectsUnknownHost covers V-17: without host
// validation, a request sent with an arbitrary Host header could poison
// links built from c.Hostname()/c.BaseURL() (e.g. password reset URLs).
func TestTrustedHostMiddleware_RejectsUnknownHost(t *testing.T) {
	app := New()
	app.Use(TrustedHostMiddleware("app.example.com"))
	app.Get("/reset", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/reset")
	ctx.Request.Header.SetHost("evil.attacker.com")

	app.Handler()(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("expected 403 for untrusted host, got %d", ctx.Response.StatusCode())
	}
}

// TestTrustedHostMiddleware_AllowsConfiguredHost verifies the normal case
// and the "*.example.com" wildcard subdomain form still work.
func TestTrustedHostMiddleware_AllowsConfiguredHost(t *testing.T) {
	app := New()
	app.Use(TrustedHostMiddleware("app.example.com", "*.wild.example.com"))
	app.Get("/reset", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	for _, host := range []string{"app.example.com", "app.example.com:8443", "sub.wild.example.com", "wild.example.com"} {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/reset")
		ctx.Request.Header.SetHost(host)

		app.Handler()(ctx)
		if ctx.Response.StatusCode() != fasthttp.StatusOK {
			t.Fatalf("host %q: expected 200, got %d", host, ctx.Response.StatusCode())
		}
	}
}

// TestTrustedHostMiddleware_NoopWhenUnconfigured keeps backward
// compatibility: apps that never call TrustedHost() are unaffected.
func TestTrustedHostMiddleware_NoopWhenUnconfigured(t *testing.T) {
	app := New()
	app.Use(TrustedHostMiddleware())
	app.Get("/x", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	ctx.Request.Header.SetHost("anything.example")

	app.Handler()(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200 when no allowedHosts configured, got %d", ctx.Response.StatusCode())
	}
}
