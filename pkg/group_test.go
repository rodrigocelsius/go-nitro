package nitro

import (
	"testing"

	"github.com/valyala/fasthttp"
)

func TestPkgGroup(t *testing.T) {
	app := New()
	group := app.Group("/api")
	group.Get("/test", func(c *Ctx) error {
		return c.SendString("ok")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/test")
	app.Handler()(ctx)

	if string(ctx.Response.Body()) != "ok" {
		t.Errorf("Body = %q, want ok", ctx.Response.Body())
	}
}

func TestPkgGroupMiddleware(t *testing.T) {
	app := New()
	var called bool

	app.Group("/api", func(c *Ctx) error {
		called = true
		return nil
	}).Get("/test", func(c *Ctx) error {
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/test")
	app.Handler()(ctx)

	if !called {
		t.Fatal("middleware not called")
	}
}

func TestPkgGroupNested(t *testing.T) {
	app := New()
	var called bool

	v1 := app.Group("/api").Group("/v1")
	v1.Get("/test", func(c *Ctx) error {
		called = true
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/test")
	app.Handler()(ctx)

	if !called {
		t.Fatal("handler not called")
	}
}
