package fasthttp_test

import (
	"bytes"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
	"time"

	nitroadapter "github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func TestBind_FormURLEncoded(t *testing.T) {
	fc := nitroadapter.NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetContentType("application/x-www-form-urlencoded")
	fctx.Request.SetBodyString("name=Ada&age=36")
	fc.Reset(fctx)

	var dest struct {
		Name string `form:"name"`
		Age  int    `form:"age"`
	}
	if err := fc.Bind(&dest); err != nil {
		t.Fatal(err)
	}
	if dest.Name != "Ada" || dest.Age != 36 {
		t.Fatalf("got %+v", dest)
	}
}

func TestBind_JSONStillWorks(t *testing.T) {
	fc := nitroadapter.NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetContentType("application/json")
	fctx.Request.SetBodyString(`{"name":"Ada"}`)
	fc.Reset(fctx)

	var dest struct {
		Name string `json:"name"`
	}
	if err := fc.Bind(&dest); err != nil {
		t.Fatal(err)
	}
	if dest.Name != "Ada" {
		t.Fatalf("got %q", dest.Name)
	}
}

func TestBodyParser_Multipart(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("title", "hello")
	_ = w.Close()

	fc := nitroadapter.NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetContentType(w.FormDataContentType())
	fctx.Request.SetBody(buf.Bytes())
	fc.Reset(fctx)

	var dest struct {
		Title string `form:"title"`
	}
	if err := fc.BodyParser(&dest); err != nil {
		t.Fatal(err)
	}
	if dest.Title != "hello" {
		t.Fatalf("got %q", dest.Title)
	}
}

func TestParam_Default(t *testing.T) {
	fc := nitroadapter.NewFastCtx()
	fc.Reset(&fasthttp.RequestCtx{})
	if got := fc.Param("id", "0"); got != "0" {
		t.Fatalf("got %q", got)
	}
	fc.SetParam("id", "7")
	if got := fc.Param("id", "0"); got != "7" {
		t.Fatalf("got %q", got)
	}
}

func TestReverseRouting(t *testing.T) {
	app := nitroadapter.New()
	app.Get("/users/:id", func(c *domain.Ctx) error { return nil }).Name("user.show")

	got := app.GetRouteURL("user.show", map[string]any{"id": 42})
	if got != "/users/42" {
		t.Fatalf("got %q", got)
	}
	if app.URL("missing") != "" {
		t.Fatal("expected empty for missing route")
	}
}

func TestMount(t *testing.T) {
	sub := nitroadapter.New()
	sub.Get("/ping", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*nitroadapter.FastCtx)
		return fc.SendString("pong")
	})

	app := nitroadapter.New()
	app.Mount("/api/v1", sub)

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/api/v1/ping")
	h(ctx)

	if got := string(ctx.Response.Body()); got != "pong" {
		t.Fatalf("got %q want pong", got)
	}
}

func TestHooks_OnRoute(t *testing.T) {
	app := nitroadapter.New()
	var paths []string
	app.Hooks().OnRoute(func(info domain.RouteInfo) error {
		paths = append(paths, info.Method+" "+info.Path)
		return nil
	})
	app.Get("/x", func(c *domain.Ctx) error { return nil })
	if len(paths) != 1 || paths[0] != "GET /x" {
		t.Fatalf("got %v", paths)
	}
}

func TestBodyParser_FormDefaultsAndTransforms(t *testing.T) {
	fc := nitroadapter.NewFastCtx()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetContentType("application/x-www-form-urlencoded")
	fctx.Request.SetBodyString("name=ada+lovelace")
	fc.Reset(fctx)

	var dest struct {
		Name   string `form:"name" transform:"titlecase=pt"`
		Active bool   `form:"active" default:"true"`
	}
	if err := fc.BodyParser(&dest); err != nil {
		t.Fatal(err)
	}
	if dest.Name != "Ada Lovelace" {
		t.Fatalf("name=%q", dest.Name)
	}
	if !dest.Active {
		t.Fatal("expected default active=true")
	}
}

func TestHooks_OnRouteNamed(t *testing.T) {
	app := nitroadapter.New()
	var names []string
	app.Hooks().OnRoute(func(info domain.RouteInfo) error {
		names = append(names, info.Name)
		return nil
	})
	app.Get("/y", func(c *domain.Ctx) error { return nil }).Name("y.show")
	if len(names) < 2 || names[len(names)-1] != "y.show" {
		t.Fatalf("OnRoute names=%v want last y.show", names)
	}
}

func TestHooks_OnShutdown(t *testing.T) {
	app := nitroadapter.New()
	called := false
	app.Hooks().OnShutdown(func() error {
		called = true
		return nil
	})
	_ = app.Shutdown()
	if !called {
		t.Fatal("OnShutdown not called")
	}
}

func TestHooks_OnListen(t *testing.T) {
	app := nitroadapter.New()
	done := make(chan string, 1)
	app.Hooks().OnListen(func(d nitroadapter.ListenData) error {
		done <- d.Addr
		return nil
	})
	go func() { _ = app.Listen("127.0.0.1:0") }()
	select {
	case addr := <-done:
		if addr != "127.0.0.1:0" {
			t.Fatalf("addr=%q", addr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("OnListen timeout")
	}
	// Best-effort stop (race: ListenAndServe may start after Shutdown if too fast).
	_ = app.Shutdown()
	time.Sleep(50 * time.Millisecond)
	_ = app.Shutdown()
}

func TestStatic_SinglePageApp(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, "index.html")
	if err := os.WriteFile(index, []byte("SPA"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := nitroadapter.New()
	app.Static("/", dir, nitroadapter.StaticConfig{SinglePageApp: true})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/app/route")
	h(ctx)

	if got := string(ctx.Response.Body()); got != "SPA" {
		t.Fatalf("got %q want SPA (status=%d)", got, ctx.Response.StatusCode())
	}
}

func TestGetRouteURL_FromContext(t *testing.T) {
	app := nitroadapter.New()
	app.Get("/items/:id", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*nitroadapter.FastCtx)
		return fc.SendString(fc.GetRouteURL("item.show", map[string]any{"id": fc.Param("id")}))
	}).Name("item.show")

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.SetRequestURI("/items/9")
	h(ctx)

	if got := string(ctx.Response.Body()); got != "/items/9" {
		t.Fatalf("got %q", got)
	}
}
