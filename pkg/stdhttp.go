package nitro

import (
	"net/http"

	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// Adapt wraps a standard net/http handler as a Nitro Handler, so the huge
// net/http ecosystem (promhttp, swaggo/http-swagger, gorilla/handlers,
// third-party webhooks, ...) works unchanged:
//
//	app.Get("/metrics", nitro.Adapt(promhttp.Handler()))
//
// The request body is fully buffered (fasthttp semantics) and the adaptor
// copies the request into net/http types, so it is slower than a native
// Nitro handler; use it for integrations, not hot paths. Response headers and
// status written by h are forwarded as-is.
func Adapt(h http.Handler) Handler {
	if h == nil {
		panic("nitro: Adapt requires a non-nil http.Handler")
	}
	fh := fasthttpadaptor.NewFastHTTPHandler(h)
	return func(c *Ctx) error {
		fh(c.FasthttpCtx())
		return nil
	}
}

// AdaptFunc is Adapt for an http.HandlerFunc-style function.
func AdaptFunc(f func(http.ResponseWriter, *http.Request)) Handler {
	return Adapt(http.HandlerFunc(f))
}
