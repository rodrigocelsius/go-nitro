package fasthttp

import (
	"context"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// TimeoutMiddleware returns a middleware that limits request execution time
// using context deadlines. Handlers observe it through c.Context() (every
// Store / HTTP client call given the request context is cancelled when the
// deadline passes).
//
// This is a cooperative timeout: the handler is not interrupted. When it
// returns after the deadline without having produced a successful response
// of its own, the client receives 503. The deadline context is released when
// the request finishes.
//
// Usage:
//
//	app.Use(TimeoutMiddleware(30 * time.Second))
//	app.Get("/slow", handler, TimeoutMiddleware(5 * time.Second))
func TimeoutMiddleware(timeout time.Duration) domain.Handler {
	if timeout <= 0 {
		return func(c *domain.Ctx) error {
			return nil
		}
	}

	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		fc.applyTimeout(timeout)
		return nil
	}
}

// applyTimeout installs a deadline on the request context. The cancel func
// runs when the request finishes (OnDone) — never when the middleware
// returns, which would hand the handler an already-cancelled context.
func (c *FastCtx) applyTimeout(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(c.RequestContext(), timeout)
	c.SetRequestContext(ctx)
	c.deadlineCtx = ctx
	c.OnDone(cancel)
}

var timeoutBody = []byte(`{"error":"request timeout","message":"request exceeded the time limit"}`)

// writeTimeoutIfExceeded turns a request that ran past its deadline into a
// 503, unless the handler already answered with something other than a
// plain 200 or the generic 500 produced by a context error.
func (c *FastCtx) writeTimeoutIfExceeded() {
	if c.deadlineCtx == nil || c.deadlineCtx.Err() != context.DeadlineExceeded {
		return
	}
	fctx := c.fasthttpReq
	if st := fctx.Response.StatusCode(); st != 200 && st != 500 {
		return
	}
	fctx.Response.ResetBody()
	fctx.SetStatusCode(503)
	fctx.Response.Header.SetContentTypeBytes(contentTypeJSON)
	fctx.SetBody(timeoutBody)
}
