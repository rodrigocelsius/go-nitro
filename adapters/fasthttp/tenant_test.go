package fasthttp

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func TestTenantID_FromLocalsAndClaims(t *testing.T) {
	c := NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	c.Reset(fctx)

	if c.TenantID() != "" {
		t.Fatal("expected empty tenant")
	}
	c.SetTenantID("t-1")
	if c.TenantID() != "t-1" {
		t.Fatalf("got %q", c.TenantID())
	}

	c2 := NewFastCtx()
	c2.Reset(&fasthttp.RequestCtx{})
	c2.Set("claims", &domain.Claims{UserID: "u", TenantID: "from-claim"})
	if c2.TenantID() != "from-claim" {
		t.Fatalf("got %q", c2.TenantID())
	}
}

func TestTenantMiddleware_FromHeader(t *testing.T) {
	app := New()
	app.Use(TenantMiddleware(TenantConfig{Require: true, AllowHeader: boolPtr(true)}))
	var got string
	app.Get("/x", func(c *domain.Ctx) error {
		got = c.AdapterData.(*FastCtx).TenantID()
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	ctx.Request.Header.Set("X-Tenant-ID", "acme")
	app.Handler()(ctx)
	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("status %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if got != "acme" {
		t.Fatalf("tenant=%q", got)
	}
}

func TestTenantMiddleware_RequireWithoutTenant(t *testing.T) {
	app := New()
	app.Use(TenantMiddleware(TenantConfig{Require: true}))
	app.Get("/x", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	app.Handler()(ctx)
	if ctx.Response.StatusCode() != 401 {
		t.Fatalf("status %d, want 401", ctx.Response.StatusCode())
	}
}

func TestTenantMiddleware_PrefersExistingLocal(t *testing.T) {
	app := New()
	app.Use(func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetTenantID("from-jwt")
		return nil
	})
	app.Use(TenantMiddleware(TenantConfig{Require: true}))
	var got string
	app.Get("/x", func(c *domain.Ctx) error {
		got = c.AdapterData.(*FastCtx).TenantID()
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	ctx.Request.Header.Set("X-Tenant-ID", "spoofed")
	app.Handler()(ctx)
	if got != "from-jwt" {
		t.Fatalf("tenant=%q, want from-jwt (header must not override)", got)
	}
}

func boolPtr(v bool) *bool { return &v }

func TestTenantMiddleware_DisallowHeader(t *testing.T) {
	app := New()
	app.Use(TenantMiddleware(TenantConfig{Require: true, AllowHeader: boolPtr(false)}))
	app.Get("/x", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	ctx.Request.Header.Set("X-Tenant-ID", "acme")
	app.Handler()(ctx)
	if ctx.Response.StatusCode() != 401 {
		t.Fatalf("status %d, want 401 when header disallowed", ctx.Response.StatusCode())
	}
}

// Regression: the header used to be trusted by default, so any unauthenticated
// caller could impersonate another tenant by sending X-Tenant-ID.
func TestTenantMiddleware_HeaderIgnoredByDefault(t *testing.T) {
	app := New()
	app.Use(TenantMiddleware())
	var got string
	app.Get("/x", func(c *domain.Ctx) error {
		got = c.AdapterData.(*FastCtx).TenantID()
		return nil
	})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	ctx.Request.Header.Set("X-Tenant-ID", "victim-tenant")
	app.Handler()(ctx)
	if got != "" {
		t.Fatalf("default config must ignore X-Tenant-ID, got tenant %q", got)
	}
}
