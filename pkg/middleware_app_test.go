package nitro

import (
	"strings"
	"testing"
	"time"
)

func TestApp_ChainableMiddlewareHelpers(t *testing.T) {
	app := New(Config{DisableDefaults: true, Env: EnvDevelopment})
	app.CORS().Compress().Timeout(time.Second).RateLimit(1000, time.Minute)
	app.Get("/big", func(c *Ctx) error { return c.Text(strings.Repeat("x", 4096)) })
	resp, err := app.TestWithHeaders("GET", "/big", nil, map[string]string{"Accept-Encoding": "gzip", "Origin": "http://localhost:3000"})
	if err != nil || resp.StatusCode() != 200 {
		t.Fatalf("%v %d", err, resp.StatusCode())
	}
	if string(resp.Header.Peek("Content-Encoding")) != "gzip" {
		t.Fatalf("app.Compress() must gzip via post-middleware; headers: %s", resp.Header.String())
	}
	if resp.Header.Peek("Access-Control-Allow-Origin") == nil {
		t.Fatalf("app.CORS() must set CORS headers in development; headers: %s", resp.Header.String())
	}
}

// app.CORS() with no AllowedOrigins answered "*" in production too, contrary
// to the documented convention (permissive only in development).
func TestApp_CORSIsEnvAware(t *testing.T) {
	hdrs := map[string]string{"Origin": "https://evil.example.com"}

	prod := New(Config{DisableDefaults: true, Env: EnvProduction})
	prod.CORS()
	prod.Get("/x", func(c *Ctx) error { return c.Text("ok") })
	resp, err := prod.TestWithHeaders("GET", "/x", nil, hdrs)
	if err != nil {
		t.Fatal(err)
	}
	if v := resp.Header.Peek("Access-Control-Allow-Origin"); v != nil {
		t.Fatalf("production app.CORS() without origins must not allow %q", v)
	}

	allowed := New(Config{DisableDefaults: true, Env: EnvProduction,
		Security: SecurityConfig{CORS: CORSConfig{AllowedOrigins: []string{"https://app.example.com"}}}})
	allowed.CORS()
	allowed.Get("/x", func(c *Ctx) error { return c.Text("ok") })
	resp, _ = allowed.TestWithHeaders("GET", "/x", nil, map[string]string{"Origin": "https://app.example.com"})
	if got := string(resp.Header.Peek("Access-Control-Allow-Origin")); got != "https://app.example.com" {
		t.Fatalf("Config.Security.CORS origins must be honored, ACAO = %q", got)
	}

	dev := New(Config{DisableDefaults: true, Env: EnvDevelopment})
	dev.CORS()
	dev.Get("/x", func(c *Ctx) error { return c.Text("ok") })
	resp, _ = dev.TestWithHeaders("GET", "/x", nil, hdrs)
	if got := string(resp.Header.Peek("Access-Control-Allow-Origin")); got != "*" {
		t.Fatalf("development app.CORS() must be permissive, ACAO = %q", got)
	}
}
