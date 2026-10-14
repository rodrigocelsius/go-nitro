package nitro

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

func TestAsCtx_RoundTrip(t *testing.T) {
	app := New(Config{DisableDefaults: true, DisableRequestID: true, Security: SecurityConfig{DisableHeaders: true}})
	app.Get("/x", func(c *Ctx) error {
		return c.Write(200, []byte(`{"ok":true}`))
	})
	app.GetDomain("/y", func(c *domain.Ctx) error {
		return AsCtx(c).Write(200, []byte(`{"ok":true}`))
	})

	h := app.Handler()
	for _, path := range []string{"/x", "/y"} {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.SetRequestURI(path)
		fctx.Request.Header.SetMethod("GET")
		h(fctx)
		if fctx.Response.StatusCode() != 200 {
			t.Fatalf("%s status = %d", path, fctx.Response.StatusCode())
		}
	}
}

func BenchmarkGetVsGetDomain(b *testing.B) {
	cfg := Config{DisableDefaults: true, DisableRequestID: true, Security: SecurityConfig{DisableHeaders: true}}
	body := []byte(`{"message":"Hello, World!"}`)

	appWrap := New(cfg)
	appWrap.Get("/", func(c *Ctx) error { return c.Write(200, body) })
	hWrap := appWrap.Handler()

	appDom := New(cfg)
	appDom.GetDomain("/", func(c *domain.Ctx) error { return AsCtx(c).Write(200, body) })
	hDom := appDom.Handler()

	b.Run("Get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.SetRequestURI("/")
			fctx.Request.Header.SetMethod("GET")
			hWrap(fctx)
		}
	})
	b.Run("GetDomain", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.SetRequestURI("/")
			fctx.Request.Header.SetMethod("GET")
			hDom(fctx)
		}
	})
}
