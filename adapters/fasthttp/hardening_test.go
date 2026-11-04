package fasthttp

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func corsRequest(t *testing.T, cfg domain.CORSConfig, method, origin string, preflight bool) *fasthttp.RequestCtx {
	t.Helper()
	app := New()
	app.Use(CORSMiddleware(cfg))
	app.Get("/x", func(c *domain.Ctx) error { return nil })
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI("/x")
	ctx.Request.Header.Set("Origin", origin)
	if preflight {
		ctx.Request.Header.Set("Access-Control-Request-Method", "GET")
	}
	app.Handler()(ctx)
	return ctx
}

// Allowlisted origins with a port or a single-label host used to be rejected
// by a hostname validator, so local front-ends never got CORS headers.
func TestCORS_AllowlistMatchesPortsAndLocalhost(t *testing.T) {
	for _, o := range []string{"http://localhost:3000", "https://app.example.com:8443", "http://localhost", "http://127.0.0.1:5173"} {
		ctx := corsRequest(t, domain.CORSConfig{AllowedOrigins: []string{o}}, "GET", o, false)
		if got := string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")); got != o {
			t.Errorf("%s: ACAO = %q, want %q", o, got, o)
		}
	}
}

func TestCORS_AllowlistNormalizesConfiguredOrigins(t *testing.T) {
	cfg := domain.CORSConfig{AllowedOrigins: []string{"https://App.Example.com/"}}
	ctx := corsRequest(t, cfg, "GET", "https://app.example.com", false)
	if got := string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")); got != "https://app.example.com" {
		t.Fatalf("ACAO = %q", got)
	}
}

func TestCORS_DisallowedOriginGetsNoHeaders(t *testing.T) {
	cfg := domain.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}, AllowCredentials: true}
	for _, preflight := range []bool{false, true} {
		method := "GET"
		if preflight {
			method = "OPTIONS"
		}
		ctx := corsRequest(t, cfg, method, "https://evil.example.com", preflight)
		for _, h := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Credentials", "Access-Control-Allow-Methods"} {
			if v := ctx.Response.Header.Peek(h); v != nil {
				t.Errorf("preflight=%v: %s = %q, want absent", preflight, h, v)
			}
		}
		if got := string(ctx.Response.Header.Peek("Vary")); got != "Origin" {
			t.Errorf("preflight=%v: Vary = %q, want Origin", preflight, got)
		}
	}
}

func TestCORS_PreflightReflectsHeadersWhenWildcard(t *testing.T) {
	app := New()
	app.Use(CORSMiddleware(domain.CORSConfig{AllowedOrigins: []string{"*"}, AllowedHeaders: []string{"*"}}))
	app.Get("/x", func(c *domain.Ctx) error { return nil })
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("OPTIONS")
	ctx.Request.SetRequestURI("/x")
	ctx.Request.Header.Set("Origin", "https://a.example.com")
	ctx.Request.Header.Set("Access-Control-Request-Method", "POST")
	ctx.Request.Header.Set("Access-Control-Request-Headers", "X-Custom")
	app.Handler()(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusNoContent {
		t.Fatalf("status = %d", ctx.Response.StatusCode())
	}
	if got := string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")); got != "*" {
		t.Fatalf("ACAO = %q", got)
	}
	if got := string(ctx.Response.Header.Peek("Access-Control-Allow-Headers")); got != "X-Custom" {
		t.Fatalf("ACAH = %q", got)
	}
}

func TestSecurityHeaders_PermissionsPolicyOverride(t *testing.T) {
	app := New(domain.Config{Security: domain.SecurityConfig{PermissionsPolicy: "camera=(self)"}})
	app.Get("/x", func(c *domain.Ctx) error { return nil })
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	app.Handler()(ctx)
	if got := string(ctx.Response.Header.Peek("Permissions-Policy")); got != "camera=(self)" {
		t.Fatalf("Permissions-Policy = %q", got)
	}
}

func TestRequestID_EchoesSafeAndReplacesUnsafe(t *testing.T) {
	app := New()
	app.Get("/x", func(c *domain.Ctx) error { return nil })
	h := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	ctx.Request.Header.Set("X-Request-ID", "req-123_ABC.9")
	h(ctx)
	if got := string(ctx.Response.Header.Peek("X-Request-ID")); got != "req-123_ABC.9" {
		t.Fatalf("safe id not echoed: %q", got)
	}

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	ctx.Request.Header.Set("X-Request-ID", "bad id\x01")
	h(ctx)
	got := string(ctx.Response.Header.Peek("X-Request-ID"))
	if got == "" || got == "bad id\x01" || !isSafeRequestID(got) {
		t.Fatalf("unsafe id must be replaced, got %q", got)
	}
}

// ListenAutoTLS used to skip Err(), so insecure configuration (e.g.
// unprotected Resources in production) did not stop the boot.
func TestListenAutoTLS_RefusesConfigErrors(t *testing.T) {
	app := New()
	want := errors.New("boom")
	app.AddConfigError(want)
	if err := app.ListenAutoTLS("127.0.0.1:0", "example.com"); !errors.Is(err, want) {
		t.Fatalf("ListenAutoTLS err = %v, want config error", err)
	}
}

// With the table full of live keys, new keys must not rescan the table on
// every request (O(maxKeys) under the global lock).
func TestPerKeySlidingWindow_FullTableSweepIsRateLimited(t *testing.T) {
	p := newPerKeySlidingWindow(10, time.Minute, 2)
	p.Allow("a")
	p.Allow("b")
	p.Allow("c") // full: sweeps once, then uses the overflow window
	first := p.lastSweep
	if first.IsZero() {
		t.Fatal("expected a sweep when the table filled up")
	}
	p.Allow("d")
	p.Allow("e")
	if !p.lastSweep.Equal(first) {
		t.Fatal("full table was swept again within maxSweepInterval")
	}
}

func TestRateLimitMiddleware_UsesCombinedLookup(t *testing.T) {
	app := New()
	app.Use(SlidingWindowRateLimitMiddleware(2, time.Minute, IPKeyFunc()))
	app.Get("/x", func(c *domain.Ctx) error { return nil })
	h := app.Handler()
	for i, want := range []string{"1", "0"} {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/x")
		h(ctx)
		if got := string(ctx.Response.Header.Peek("X-RateLimit-Remaining")); got != want {
			t.Fatalf("req %d: remaining = %q, want %q", i, got, want)
		}
	}
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	h(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusTooManyRequests {
		t.Fatalf("3rd request status = %d, want 429", ctx.Response.StatusCode())
	}
}

// Compress overwrote "Vary: Origin" set by CORS, letting a shared cache serve
// one origin's Access-Control-Allow-Origin to another.
func TestVary_CORSAndCompressAreMerged(t *testing.T) {
	app := New()
	app.Use(CORSMiddleware(domain.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}}))
	app.UsePost(CompressMiddleware())
	app.Get("/x", func(c *domain.Ctx) error {
		return c.AdapterData.(*FastCtx).SendString(strings.Repeat("x", 4096))
	})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/x")
	ctx.Request.Header.Set("Origin", "https://app.example.com")
	ctx.Request.Header.Set("Accept-Encoding", "gzip")
	app.Handler()(ctx)
	if got := string(ctx.Response.Header.Peek("Vary")); got != "Origin, Accept-Encoding" {
		t.Fatalf("Vary = %q, want both tokens", got)
	}
	h := &fasthttp.ResponseHeader{}
	h.Set("Vary", "origin")
	appendVary(h, []byte("Origin"))
	if got := string(h.Peek("Vary")); got != "origin" {
		t.Fatalf("duplicate token appended: %q", got)
	}
}
