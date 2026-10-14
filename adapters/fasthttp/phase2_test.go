package fasthttp

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/adapters/ratelimit/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// =============================================================================
// Tests 2.1: Security Headers
// =============================================================================

func TestSecurityHeaders_AllPresent(t *testing.T) {
	app := New()
	app.Get("/test", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	tests := []struct {
		header string
		want   string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Content-Security-Policy", "default-src 'self'"},
		{"X-XSS-Protection", "1; mode=block"},
		{"X-Permitted-Cross-Domain-Policies", "none"},
		{"Permissions-Policy", "none"},
	}

	for _, tt := range tests {
		got := string(ctx.Response.Header.Peek(tt.header))
		if got != tt.want {
			t.Errorf("Header %s = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestSecurityHeaders_HSTSEnabled(t *testing.T) {
	app := New() // Default: EnvProduction
	app.Get("/test", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	hsts := string(ctx.Response.Header.Peek("Strict-Transport-Security"))
	if hsts != "max-age=31536000; includeSubDomains; preload" {
		t.Errorf("HSTS = %q, want max-age=31536000; includeSubDomains; preload", hsts)
	}
}

func TestSecurityHeaders_HSTSDisabledInDev(t *testing.T) {
	app := New(domain.Config{Env: domain.EnvDevelopment})
	app.Get("/test", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	hsts := string(ctx.Response.Header.Peek("Strict-Transport-Security"))
	if hsts != "" {
		t.Errorf("HSTS should be empty in dev mode, got %q", hsts)
	}
}

func TestSecurityHeaders_Disabled(t *testing.T) {
	app := New(domain.Config{
		Security: domain.SecurityConfig{DisableHeaders: true},
	})
	app.Get("/test", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	if len(ctx.Response.Header.Peek("X-Content-Type-Options")) > 0 {
		t.Error("Security headers should be disabled")
	}
}

func TestSecurityHeaders_CustomCSP(t *testing.T) {
	csp := "default-src 'self'; script-src 'self' 'unsafe-inline'"
	app := New(domain.Config{
		Security: domain.SecurityConfig{CSP: csp},
	})
	app.Get("/test", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	got := string(ctx.Response.Header.Peek("Content-Security-Policy"))
	if got != csp {
		t.Errorf("CSP = %q, want %q", got, csp)
	}
}

func BenchmarkSecurityHeaders(b *testing.B) {
	app := New()
	app.Get("/test", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	handler := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler(ctx)
	}
}

// =============================================================================
// Tests 2.5: Rate Limiting Headers
// =============================================================================

func TestRateLimitMiddleware_Headers(t *testing.T) {
	limiter := ratelimit.NewMemoryLimiter(10, time.Minute) // 10 req/min
	app := New()
	app.Use(RateLimitMiddleware(limiter, 10, IPKeyFunc()))
	app.Get("/test", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	handler := app.Handler()

	// Make 5 requests and check that remaining decrements
	for i := 0; i < 5; i++ {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/test")
		handler(ctx)

		limit := string(ctx.Response.Header.Peek("X-RateLimit-Limit"))
		remaining := string(ctx.Response.Header.Peek("X-RateLimit-Remaining"))

		if limit != "10" {
			t.Errorf("X-RateLimit-Limit = %q, want 10", limit)
		}
		expected := 10 - (i + 1)
		if remaining != fmt.Sprintf("%d", expected) {
			t.Errorf("req %d: X-RateLimit-Remaining = %q, want %d", i, remaining, expected)
		}
	}
}

func TestRateLimitMiddleware_Exceeded(t *testing.T) {
	limiter := ratelimit.NewMemoryLimiter(2, time.Minute) // 2 req/min
	app := New()
	app.Use(RateLimitMiddleware(limiter, 2, IPKeyFunc()))
	app.Get("/test", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	handler := app.Handler()

	// Make 3 requests (must block on the 3rd)
	for i := 0; i < 3; i++ {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/test")
		handler(ctx)

		if i == 2 {
			if ctx.Response.StatusCode() != 429 {
				t.Errorf("StatusCode = %d, want 429", ctx.Response.StatusCode())
			}
			remaining := string(ctx.Response.Header.Peek("X-RateLimit-Remaining"))
			if remaining != "0" {
				t.Errorf("X-RateLimit-Remaining = %q, want 0", remaining)
			}
		}
	}
}

func BenchmarkRateLimitMiddleware(b *testing.B) {
	limiter := ratelimit.NewMemoryLimiter(1000000, time.Minute)
	app := New()
	app.Use(RateLimitMiddleware(limiter, 1000000, IPKeyFunc()))
	app.Get("/test", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	handler := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler(ctx)
	}
}

// =============================================================================
// Tests 2.6: Request Size Limits
// =============================================================================

func TestMaxBodySizeMiddleware_Allowed(t *testing.T) {
	app := New()
	app.Use(MaxBodySizeMiddleware(1024))
	app.Post("/upload", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	handler := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/upload")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentLength(100)
	handler(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", ctx.Response.StatusCode())
	}
}

func TestMaxBodySizeMiddleware_Rejected(t *testing.T) {
	app := New()
	app.Use(MaxBodySizeMiddleware(100)) // 100 bytes
	app.Post("/upload", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	handler := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/upload")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentLength(200) // 200 bytes > 100 limit
	handler(ctx)

	if ctx.Response.StatusCode() != 413 {
		t.Errorf("StatusCode = %d, want 413", ctx.Response.StatusCode())
	}
}

func BenchmarkMaxBodySizeMiddleware(b *testing.B) {
	app := New()
	app.Use(MaxBodySizeMiddleware(1024))
	app.Post("/upload", func(c *domain.Ctx) error {
		c.SetBody([]byte("ok"))
		return nil
	})

	handler := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/upload")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentLength(100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler(ctx)
	}
}

// =============================================================================
// Tests 2.2: Fuzz Testing (manual execution)
// =============================================================================

func TestFuzzRouter_Seed(t *testing.T) {
	// Manual test with known seeds
	rt := NewRadixRouter()
	handler := func(_ *domain.Ctx) error { return nil }

	rt.Add("GET", "/health", handler)
	rt.Add("GET", "/users/:id", handler)

	seeds := []string{
		"/health",
		"/users/123",
		"/../../../etc/passwd",
		"",
		string(make([]byte, 10000)),
	}

	for _, seed := range seeds {
		c := &domain.Ctx{}
		// Must not panic
		rt.Lookup([]byte("GET"), []byte(seed), c)
	}
}

func TestFuzzBinding_Seed(t *testing.T) {
	// Manual test with known seeds
	seeds := []string{
		"123",
		"abc",
		"",
		"99999999999999999999",
		"-1",
		"0",
		"1.5",
		"true",
		"false",
	}

	for _, seed := range seeds {
		// Must not panic
		convertAndSet(reflect.ValueOf(new(int)).Elem(), seed)
		convertAndSet(reflect.ValueOf(new(string)).Elem(), seed)
	}
}
