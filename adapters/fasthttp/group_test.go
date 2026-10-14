package fasthttp

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func TestGroupPrefix(t *testing.T) {
	app := New()
	var called bool
	group := app.Group("/api")
	group.Get("/users", func(c *domain.Ctx) error {
		called = true
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/users")
	app.Handler()(ctx)

	if !called {
		t.Fatal("handler not called")
	}
	if ctx.Response.StatusCode() != 200 {
		t.Errorf("StatusCode = %d, want 200", ctx.Response.StatusCode())
	}
}

func TestGroupMiddleware(t *testing.T) {
	app := New()
	var mwCalled bool
	var handlerCalled bool

	group := app.Group("/api", func(c *domain.Ctx) error {
		mwCalled = true
		return nil
	})

	group.Get("/test", func(c *domain.Ctx) error {
		handlerCalled = true
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/test")
	app.Handler()(ctx)

	if !mwCalled {
		t.Fatal("middleware not called")
	}
	if !handlerCalled {
		t.Fatal("handler not called")
	}
}

func TestGroupNested(t *testing.T) {
	app := New()
	var called bool

	v1 := app.Group("/api").Group("/v1")
	v1.Get("/users", func(c *domain.Ctx) error {
		called = true
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/users")
	app.Handler()(ctx)

	if !called {
		t.Fatal("handler not called")
	}
}

func TestGroupNestedMiddleware(t *testing.T) {
	app := New()
	var outerCalled bool
	var innerCalled bool

	outer := app.Group("/api", func(c *domain.Ctx) error {
		outerCalled = true
		return nil
	})
	inner := outer.Group("/v1", func(c *domain.Ctx) error {
		innerCalled = true
		return nil
	})
	inner.Get("/test", func(c *domain.Ctx) error {
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/test")
	app.Handler()(ctx)

	if !outerCalled {
		t.Fatal("outer middleware not called")
	}
	if !innerCalled {
		t.Fatal("inner middleware not called")
	}
}

func TestGroupMethods(t *testing.T) {
	app := New()
	methods := map[string]bool{
		"GET":     false,
		"POST":    false,
		"PUT":     false,
		"DELETE":  false,
		"PATCH":   false,
		"HEAD":    false,
		"OPTIONS": false,
	}

	group := app.Group("/api")
	for method := range methods {
		m := method
		switch m {
		case "GET":
			group.Get("/test", func(c *domain.Ctx) error { methods[m] = true; return nil })
		case "POST":
			group.Post("/test", func(c *domain.Ctx) error { methods[m] = true; return nil })
		case "PUT":
			group.Put("/test", func(c *domain.Ctx) error { methods[m] = true; return nil })
		case "DELETE":
			group.Delete("/test", func(c *domain.Ctx) error { methods[m] = true; return nil })
		case "PATCH":
			group.Patch("/test", func(c *domain.Ctx) error { methods[m] = true; return nil })
		case "HEAD":
			group.Head("/test", func(c *domain.Ctx) error { methods[m] = true; return nil })
		case "OPTIONS":
			group.Options("/test", func(c *domain.Ctx) error { methods[m] = true; return nil })
		}
	}

	for method, _ := range methods {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/api/test")
		ctx.Request.Header.SetMethod(method)
		app.Handler()(ctx)
	}

	for method, called := range methods {
		if !called {
			t.Errorf("handler %s /api/test not called", method)
		}
	}
}

func TestGroupRoutes(t *testing.T) {
	app := New()
	group := app.Group("/api")
	group.Get("/users", func(c *domain.Ctx) error { return nil })
	group.Post("/users", func(c *domain.Ctx) error { return nil })

	routes := app.Routes()
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	paths := map[string]bool{}
	for _, r := range routes {
		paths[r.Method+" "+r.Path] = true
	}

	if !paths["GET /api/users"] {
		t.Error("GET /api/users not found")
	}
	if !paths["POST /api/users"] {
		t.Error("POST /api/users not found")
	}
}

func TestGroupUse(t *testing.T) {
	app := New()
	var order []string

	group := app.Group("/api")
	group.Use(func(c *domain.Ctx) error {
		order = append(order, "group-mw")
		return nil
	})

	group.Get("/test", func(c *domain.Ctx) error {
		order = append(order, "handler")
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/test")
	app.Handler()(ctx)

	if len(order) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(order))
	}
	if order[0] != "group-mw" || order[1] != "handler" {
		t.Errorf("wrong order: %v", order)
	}
}

func TestGroupWithRouteMiddleware(t *testing.T) {
	app := New()
	var order []string

	group := app.Group("/api", func(c *domain.Ctx) error {
		order = append(order, "group")
		return nil
	})

	group.Get("/test", func(c *domain.Ctx) error {
		order = append(order, "handler")
		return nil
	}, func(c *domain.Ctx) error {
		order = append(order, "route")
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/test")
	app.Handler()(ctx)

	if len(order) != 3 {
		t.Fatalf("expected 3 calls, got %d: %v", len(order), order)
	}
	if order[0] != "group" || order[1] != "route" || order[2] != "handler" {
		t.Errorf("wrong order: %v (want [group route handler])", order)
	}
}

func TestGroupResponse(t *testing.T) {
	app := New()
	group := app.Group("/api")
	group.Get("/hello", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		return fc.SendString("world")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/hello")
	app.Handler()(ctx)

	if string(ctx.Response.Body()) != "world" {
		t.Errorf("Body = %q, want world", ctx.Response.Body())
	}
}
