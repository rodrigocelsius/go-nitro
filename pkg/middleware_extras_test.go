package nitro

import (
	"encoding/base64"
	"testing"
)

func extrasApp() *App {
	app := New(Config{DisableDefaults: true, Env: EnvDevelopment})
	app.ETag().ResponseTime()
	app.Get("/hello", func(c *Ctx) error { return c.Text("hi") })
	app.Get("/secret", func(c *Ctx) error { return c.Text("s") }, APIKeyAuth("k1"))
	app.Get("/admin", func(c *Ctx) error { return c.Text("a") }, BasicAuth(map[string]string{"u": "p"}))
	app.Get("/r", func(c *Ctx) error { return c.Text("r") }, RedirectHTTPS())
	app.Get("/skipped", func(c *Ctx) error { return c.Text("ok") },
		Skip(func(c *Ctx) bool { return true }, func(c *Ctx) error { return Unauthorized() }))
	return app
}

func TestExtras_ETagAndTiming(t *testing.T) {
	app := extrasApp()
	r, err := app.Test("GET", "/hello", nil)
	if err != nil || r.StatusCode() != 200 {
		t.Fatalf("%v", err)
	}
	tag := string(r.Header.Peek("ETag"))
	if tag == "" || len(r.Header.Peek("Server-Timing")) == 0 {
		t.Fatalf("missing ETag/Server-Timing: %s", r.Header.String())
	}
	r2, _ := app.TestWithHeaders("GET", "/hello", nil, map[string]string{"If-None-Match": tag})
	if r2.StatusCode() != 304 {
		t.Fatalf("want 304, got %d", r2.StatusCode())
	}
}

func TestExtras_Auth(t *testing.T) {
	app := extrasApp()
	if r, _ := app.Test("GET", "/secret", nil); r.StatusCode() != 401 {
		t.Fatalf("api key missing: %d", r.StatusCode())
	}
	if r, _ := app.TestWithHeaders("GET", "/secret", nil, map[string]string{"X-API-Key": "k1"}); r.StatusCode() != 200 {
		t.Fatalf("api key: %d", r.StatusCode())
	}
	if r, _ := app.TestWithHeaders("GET", "/secret", nil, map[string]string{"Authorization": "Bearer k1"}); r.StatusCode() != 200 {
		t.Fatalf("bearer: %d", r.StatusCode())
	}
	if r, _ := app.TestWithHeaders("GET", "/secret", nil, map[string]string{"X-API-Key": "bad"}); r.StatusCode() != 401 {
		t.Fatalf("bad key: %d", r.StatusCode())
	}
	if r, _ := app.Test("GET", "/admin", nil); r.StatusCode() != 401 || len(r.Header.Peek("WWW-Authenticate")) == 0 {
		t.Fatalf("basic missing: %d", r.StatusCode())
	}
	ok := "Basic " + base64.StdEncoding.EncodeToString([]byte("u:p"))
	if r, _ := app.TestWithHeaders("GET", "/admin", nil, map[string]string{"Authorization": ok}); r.StatusCode() != 200 {
		t.Fatalf("basic ok: %d", r.StatusCode())
	}
	bad := "Basic " + base64.StdEncoding.EncodeToString([]byte("u:x"))
	if r, _ := app.TestWithHeaders("GET", "/admin", nil, map[string]string{"Authorization": bad}); r.StatusCode() != 401 {
		t.Fatalf("basic bad: %d", r.StatusCode())
	}
}

func TestExtras_SkipAndRedirect(t *testing.T) {
	app := extrasApp()
	if r, _ := app.Test("GET", "/skipped", nil); r.StatusCode() != 200 {
		t.Fatalf("skip: %d", r.StatusCode())
	}
	// loopback is never redirected
	if r, _ := app.Test("GET", "/r", nil); r.StatusCode() != 200 {
		t.Fatalf("loopback redirect: %d %s", r.StatusCode(), r.Body())
	}
	r, _ := app.TestWithHeaders("GET", "/r", nil, map[string]string{"Host": "example.com"})
	if r.StatusCode() != 308 || string(r.Header.Peek("Location")) != "https://example.com/r" {
		t.Fatalf("redirect: %d %s", r.StatusCode(), r.Header.Peek("Location"))
	}
}
