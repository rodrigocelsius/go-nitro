package routetestdata

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/v2"
)

func newBenchAppTyped() *nitro.App {
	app := nitro.New()
	app.Get("/users/:id", GetUser_NitroHandler)
	return app
}

func newBenchAppUntyped() *nitro.App {
	app := nitro.New()
	app.Get("/users/:id", func(c *nitro.Ctx) error {
		id := c.Param("id")
		return c.SendString("user-" + id)
	})
	return app
}

func BenchmarkGeneratedRoute_TypedParam(b *testing.B) {
	app := newBenchAppTyped()
	handler := app.Handler()
	ctx := newFastCtx("GET", "/users/42")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod("GET")
		ctx.Request.SetRequestURI("/users/42")
		handler(ctx)
	}
}

func BenchmarkGeneratedRoute_UntypedParam(b *testing.B) {
	app := newBenchAppUntyped()
	handler := app.Handler()
	ctx := newFastCtx("GET", "/users/42")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod("GET")
		ctx.Request.SetRequestURI("/users/42")
		handler(ctx)
	}
}
