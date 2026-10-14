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
