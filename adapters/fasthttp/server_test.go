package fasthttp

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func TestNew_App(t *testing.T) {
	app := New()
	if app == nil {
		t.Fatal("New() returned nil")
	}
	if app.router == nil {
		t.Error("router not initialized")
	}
}

// TestFastHTTPServer_MaxRequestBodySize verifies V-15: the underlying
// fasthttp.Server enforces Config.MaxRequestBodySize at the connection
// level (before any handler/middleware runs), and falls back to
// fasthttp's own default when unset.
func TestFastHTTPServer_MaxRequestBodySize(t *testing.T) {
	app := New(domain.Config{MaxRequestBodySize: 2048})
	srv := app.FastHTTPServer()
	if srv.MaxRequestBodySize != 2048 {
		t.Fatalf("MaxRequestBodySize = %d, want 2048", srv.MaxRequestBodySize)
	}

	appDefault := New()
	srvDefault := appDefault.FastHTTPServer()
	if srvDefault.MaxRequestBodySize != DefaultMaxRequestBodySize {
		t.Fatalf("default MaxRequestBodySize = %d, want %d", srvDefault.MaxRequestBodySize, DefaultMaxRequestBodySize)
	}
}

// TestWarnPlainHTTPInProduction covers V-32: booting with Env=production
// over plain HTTP should print a stderr warning, since the framework
// cannot tell a forgotten TLS setup apart from TLS legitimately being
// terminated by an external proxy.
func TestWarnPlainHTTPInProduction(t *testing.T) {
	captureStderr := func(fn func()) string {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		orig := os.Stderr
		os.Stderr = w
		fn()
		w.Close()
		os.Stderr = orig
		out, _ := io.ReadAll(r)
		return string(out)
	}

	if out := captureStderr(func() { warnPlainHTTPInProduction(domain.EnvProduction) }); !strings.Contains(out, "plain HTTP") {
		t.Fatalf("expected warning for production plain HTTP, got %q", out)
	}
	if out := captureStderr(func() { warnPlainHTTPInProduction(domain.EnvDevelopment) }); out != "" {
		t.Fatalf("expected no warning in development, got %q", out)
	}
}

// TestDeleteCookie_SecureInferredFromScheme covers V-30: DeleteCookie
// must not hardcode Secure=false — it should follow the request's own
// scheme, like the CSRF/OAuth cookie helpers already do.
func TestDeleteCookie_SecureInferredFromScheme(t *testing.T) {
	app := New()
	app.Get("/logout", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).DeleteCookie("session_id")
		return nil
	})

	httpsCtx := &fasthttp.RequestCtx{}
	httpsCtx.Request.SetRequestURI("https://example.com/logout")
	httpsCtx.Request.Header.SetHost("example.com")
	httpsCtx.Request.URI().SetScheme("https")
	app.Handler()(httpsCtx)
	if !strings.Contains(strings.ToLower(string(httpsCtx.Response.Header.Peek("Set-Cookie"))), "secure") {
		t.Fatalf("expected Secure attribute on HTTPS request, got %q", httpsCtx.Response.Header.Peek("Set-Cookie"))
	}

	httpCtx := &fasthttp.RequestCtx{}
	httpCtx.Request.SetRequestURI("/logout")
	app.Handler()(httpCtx)
	if strings.Contains(strings.ToLower(string(httpCtx.Response.Header.Peek("Set-Cookie"))), "secure") {
		t.Fatalf("expected no Secure attribute on plain HTTP request, got %q", httpCtx.Response.Header.Peek("Set-Cookie"))
	}
}

func TestApp_Get(t *testing.T) {
	app := New()

	app.Get("/test", func(c *domain.Ctx) error {
		return nil
	})

	routes := app.Routes()
	if len(routes) != 1 {
		t.Fatalf("Routes() = %d, want 1", len(routes))
	}
	if routes[0].Method != "GET" || routes[0].Path != "/test" {
		t.Errorf("Route = %v, want GET /test", routes[0])
	}
}

func TestApp_Post(t *testing.T) {
	app := New()
	app.Post("/create", func(c *domain.Ctx) error { return nil })

	routes := app.Routes()
	if len(routes) != 1 {
		t.Fatalf("Routes() = %d, want 1", len(routes))
	}
	if routes[0].Method != "POST" {
		t.Errorf("Method = %q, want POST", routes[0].Method)
	}
}

func TestApp_Put(t *testing.T) {
	app := New()
	app.Put("/update", func(c *domain.Ctx) error { return nil })

	routes := app.Routes()
	if routes[0].Method != "PUT" {
		t.Errorf("Method = %q, want PUT", routes[0].Method)
	}
}

func TestApp_Delete(t *testing.T) {
	app := New()
	app.Delete("/remove", func(c *domain.Ctx) error { return nil })

	routes := app.Routes()
	if routes[0].Method != "DELETE" {
		t.Errorf("Method = %q, want DELETE", routes[0].Method)
	}
}

func TestApp_Handler_StaticRoute(t *testing.T) {
	app := New()
	app.Get("/hello", func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok {
			t.Fatal("AdapterData is not *FastCtx")
		}
		return fc.SendString("world")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/hello")
	app.Handler()(ctx)

	if string(ctx.Response.Body()) != "world" {
		t.Errorf("Body = %q, want world", ctx.Response.Body())
	}
}

func TestApp_Handler_ParamRoute(t *testing.T) {
	app := New()
	app.Get("/users/:id", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		return fc.SendString("user-" + c.Param("id"))
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/users/42")
	app.Handler()(ctx)

	if string(ctx.Response.Body()) != "user-42" {
		t.Errorf("Body = %q, want user-42", ctx.Response.Body())
	}
}

func TestApp_Handler_NotFound(t *testing.T) {
	app := New()
	app.Get("/exists", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/missing")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 404 {
		t.Errorf("StatusCode = %d, want 404", ctx.Response.StatusCode())
	}
}

func TestApp_Handler_MethodNotAllowed(t *testing.T) {
	app := New()
	app.Get("/data", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/data")
	ctx.Request.Header.SetMethod("POST")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 405 {
		t.Errorf("StatusCode = %d, want 405 for wrong method", ctx.Response.StatusCode())
	}
	if got := string(ctx.Response.Header.Peek("Allow")); got != "GET, OPTIONS" {
		t.Errorf("Allow = %q, want \"GET, OPTIONS\"", got)
	}
}

// A path that exists under several methods (dynamic segment) lists them all;
// an unknown path stays 404 without an Allow header.
func TestApp_Handler_MethodNotAllowed_DynamicPath(t *testing.T) {
	app := New()
	app.Get("/items/:id", func(c *domain.Ctx) error { return nil })
	app.Patch("/items/:id", func(c *domain.Ctx) error { return nil })
	app.Delete("/items/:id", func(c *domain.Ctx) error { return nil })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/items/42")
	ctx.Request.Header.SetMethod("PUT")
	app.Handler()(ctx)
	if ctx.Response.StatusCode() != 405 {
		t.Fatalf("StatusCode = %d, want 405", ctx.Response.StatusCode())
	}
	if got := string(ctx.Response.Header.Peek("Allow")); got != "DELETE, GET, OPTIONS, PATCH" {
		t.Errorf("Allow = %q", got)
	}

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/nope/42")
	ctx.Request.Header.SetMethod("PUT")
	app.Handler()(ctx)
	if ctx.Response.StatusCode() != 404 || len(ctx.Response.Header.Peek("Allow")) != 0 {
		t.Errorf("unknown path: status=%d allow=%q", ctx.Response.StatusCode(), ctx.Response.Header.Peek("Allow"))
	}
}

func TestApp_Use_GlobalMiddleware(t *testing.T) {
	app := New()
	var order []string

	app.Use(func(c *domain.Ctx) error {
		order = append(order, "pre")
		return nil
	})

	app.Get("/test", func(c *domain.Ctx) error {
		order = append(order, "handler")
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	if len(order) != 2 || order[0] != "pre" || order[1] != "handler" {
		t.Errorf("Middleware order = %v, want [pre handler]", order)
	}
}

func TestApp_UsePost_GlobalMiddleware(t *testing.T) {
	app := New()
	var order []string

	app.UsePost(func(c *domain.Ctx) error {
		order = append(order, "post")
		return nil
	})

	app.Get("/test", func(c *domain.Ctx) error {
		order = append(order, "handler")
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test")
	app.Handler()(ctx)

	if len(order) != 2 || order[0] != "handler" || order[1] != "post" {
		t.Errorf("Order = %v, want [handler post]", order)
	}
}

func TestApp_RouteName(t *testing.T) {
	app := New()
	app.Get("/named", func(c *domain.Ctx) error { return nil }).Name("getUser")

	routes := app.Routes()
	if len(routes) != 1 {
		t.Fatal("Routes() empty")
	}
	if routes[0].Name != "getUser" {
		t.Errorf("Name = %q, want getUser", routes[0].Name)
	}
}

func TestApp_UseLinearRouter(t *testing.T) {
	app := New(domain.Config{UseLinearRouter: true})
	app.Get("/linear", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		return fc.SendString("ok")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/linear")
	app.Handler()(ctx)

	if string(ctx.Response.Body()) != "ok" {
		t.Errorf("Body = %q, want ok (linear router)", ctx.Response.Body())
	}
}

func TestApp_Shutdown(t *testing.T) {
	app := New()
	err := app.Shutdown()
	if err != nil {
		t.Errorf("Shutdown on nil server should not error, got: %v", err)
	}
}

func helperApp() *App {
	return New()
}

func TestApp_Handler_HeadersPreserved(t *testing.T) {
	app := helperApp()
	app.Get("/headers", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		fc.FasthttpCtx().Response.Header.Set("X-Custom", "test-value")
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/headers")
	app.Handler()(ctx)

	if ctx.Response.Header.Peek("X-Custom") == nil {
		t.Error("Custom header not preserved")
	}
}

func TestApp_Handler_ErrorReturns500(t *testing.T) {
	app := New()
	app.Get("/error", func(c *domain.Ctx) error {
		return fmt.Errorf("something went wrong")
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/error")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 500 {
		t.Errorf("StatusCode = %d, want 500 on handler error", ctx.Response.StatusCode())
	}
}

func TestApp_MultipleRoutes(t *testing.T) {
	app := New()
	app.Get("/a", func(c *domain.Ctx) error { return nil })
	app.Get("/b", func(c *domain.Ctx) error { return nil })
	app.Post("/a", func(c *domain.Ctx) error { return nil })

	routes := app.Routes()
	if len(routes) != 3 {
		t.Errorf("Routes() = %d, want 3", len(routes))
	}
}

func BenchmarkApp_HandlerStatic(b *testing.B) {
	app := New()
	app.Get("/bench", func(c *domain.Ctx) error {
		return c.AdapterData.(*FastCtx).SendString("ok")
	})

	handler := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/bench")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		handler(ctx)
	}
}
