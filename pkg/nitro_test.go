package nitro

import (
	"testing"

	"github.com/valyala/fasthttp"
)

func newTestCtx(method, path string, body []byte) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI(path)
	if body != nil {
		ctx.Request.SetBody(body)
	}
	return ctx
}

func TestApp_StaticRoute(t *testing.T) {
	app := New()
	app.Get("/health", func(c *Ctx) error {
		return c.SendString("ok")
	})

	handler := app.Handler()
	ctx := newTestCtx("GET", "/health", nil)
	handler(ctx)

	if got := string(ctx.Response.Body()); got != "ok" {
		t.Fatalf("expected body %q, got %q", "ok", got)
	}
}

func TestApp_ParamRoute(t *testing.T) {
	app := New()
	app.Get("/users/:id", func(c *Ctx) error {
		return c.SendString("user:" + c.Param("id"))
	})

	handler := app.Handler()
	ctx := newTestCtx("GET", "/users/42", nil)
	handler(ctx)

	if got := string(ctx.Response.Body()); got != "user:42" {
		t.Fatalf("expected body %q, got %q", "user:42", got)
	}
}

func TestApp_NotFound(t *testing.T) {
	app := New()
	app.Get("/health", func(c *Ctx) error {
		return c.SendString("ok")
	})

	handler := app.Handler()
	ctx := newTestCtx("GET", "/nope", nil)
	handler(ctx)

	if ctx.Response.StatusCode() != 404 {
		t.Fatalf("expected status 404, got %d", ctx.Response.StatusCode())
	}
}

func TestApp_MethodMismatch(t *testing.T) {
	app := New()
	app.Get("/users", func(c *Ctx) error {
		return c.SendString("ok")
	})

	handler := app.Handler()
	ctx := newTestCtx("POST", "/users", nil)
	handler(ctx)

	if ctx.Response.StatusCode() != 405 {
		t.Fatalf("expected status 405 for method mismatch, got %d", ctx.Response.StatusCode())
	}
}

func TestApp_MultipleParams(t *testing.T) {
	app := New()
	app.Get("/users/:userId/posts/:postId", func(c *Ctx) error {
		return c.SendString(c.Param("userId") + "/" + c.Param("postId"))
	})

	handler := app.Handler()
	ctx := newTestCtx("GET", "/users/7/posts/99", nil)
	handler(ctx)

	if got := string(ctx.Response.Body()); got != "7/99" {
		t.Fatalf("expected body %q, got %q", "7/99", got)
	}
}
