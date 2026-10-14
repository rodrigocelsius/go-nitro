package fasthttp

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

type perfUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func perfApp() func(*fasthttp.RequestCtx) { return perfAppCfg(domain.Config{}) }

func perfAppCfg(cfg domain.Config) func(*fasthttp.RequestCtx) {
	app := New(cfg)
	app.Get("/ping", func(c *domain.Ctx) error { return c.AdapterData.(*FastCtx).SendString("pong") })
	app.Get("/users/:id", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		return fc.JSON(200, perfUser{ID: fc.Param("id"), Name: "Ana", Email: "ana@x.com"})
	})
	app.Post("/echo", func(c *domain.Ctx) error {
		fc := c.AdapterData.(*FastCtx)
		var in perfUser
		if err := fc.BodyJSON(&in); err != nil {
			return err
		}
		return fc.JSON(200, in)
	})
	return app.Handler()
}

func benchPerf(b *testing.B, method, uri, body string) {
	benchPerfH(b, perfApp(), method, uri, body)
}

func benchPerfH(b *testing.B, h func(*fasthttp.RequestCtx), method, uri, body string) {
	ctx := &fasthttp.RequestCtx{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(method)
		ctx.Request.SetRequestURI(uri)
		if body != "" {
			ctx.Request.Header.SetContentType("application/json")
			ctx.Request.SetBodyString(body)
		}
		h(ctx)
	}
}

func BenchmarkPerfPing(b *testing.B) { benchPerf(b, "GET", "/ping", "") }
func BenchmarkPerfUser(b *testing.B) { benchPerf(b, "GET", "/users/42", "") }
func BenchmarkPerfEcho(b *testing.B) {
	benchPerf(b, "POST", "/echo", `{"name":"Ana","email":"a@x.com"}`)
}

func BenchmarkPerfPingBare(b *testing.B) {
	h := perfAppCfg(domain.Config{DisableRequestID: true, Security: domain.SecurityConfig{DisableHeaders: true}})
	benchPerfH(b, h, "GET", "/ping", "")
}

func BenchmarkPerfPingFullStack(b *testing.B) {
	app := New()
	app.Use(RecoverMiddlewareWithDebug(false))
	pre, post := Logger(LoggerConfig{SkipRoutes: DefaultSkipRoutes, Async: true, Colors: "never"})
	app.Use(pre)
	app.UsePost(post)
	app.Use(MaxBodySizeMiddleware(1 << 20))
	app.Use(RequestIDMiddleware())
	app.Use(SecurityHeadersMiddleware())
	app.Get("/ping", func(c *domain.Ctx) error { return c.AdapterData.(*FastCtx).SendString("pong") })
	benchPerfH(b, app.Handler(), "GET", "/ping", "")
}
