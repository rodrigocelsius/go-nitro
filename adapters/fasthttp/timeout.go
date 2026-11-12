package fasthttp

import (
	"context"
	"errors"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// TimeoutMiddleware returns a middleware that limits request execution time
// using context deadlines. Handlers observe it through c.Context() (every
// Store / HTTP client call given the request context is cancelled when the
// deadline passes).
//
// This is a cooperative timeout: the handler is not interrupted. When it
// returns after the deadline with a context error, a generic error or no
// response at all, the client receives 503; a response the handler did write
// is kept. The deadline context is released when the request finishes.
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

// requestTimeoutError is the 503 answered when a request ran past its
// deadline without producing a response of its own.
type requestTimeoutError struct{}

func (requestTimeoutError) Error() string   { return "request timeout" }
func (requestTimeoutError) StatusCode() int { return 503 }
func (requestTimeoutError) Body() any {
	return map[string]string{"error": "request timeout", "message": "request exceeded the time limit"}
}

var errRequestTimeout error = requestTimeoutError{}

// timeoutAware makes the route handler report a missed deadline as an
// error, so the status is known before the post-middlewares run (the access
// log and the tracing span record the 503 the client receives).
func timeoutAware(h domain.Handler) domain.Handler {
	return func(c *domain.Ctx) error {
		err := h(c)
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil || fc.deadlineCtx == nil {
			return err
		}
		return fc.timeoutResult(err)
	}
}

// timeoutResult decides what a handler that returned after the deadline
// answers. A handler that completed and wrote a response keeps it — its
// side effects happened, and a 503 would invite the client to repeat them.
// The 503 replaces only an outcome caused by the deadline: a context error,
// a generic (non-HTTP or 500) error, or no response at all.
func (c *FastCtx) timeoutResult(err error) error {
	if c.deadlineCtx.Err() != context.DeadlineExceeded {
		return err
	}
	if err != nil {
		if err == domain.ErrAbort {
			return err
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return errRequestTimeout
		}
		if httpErr, ok := err.(domain.HTTPError); ok && httpErr.StatusCode() != 500 {
			return err
		}
		return errRequestTimeout
	}
	resp := &c.fasthttpReq.Response
	if resp.StatusCode() == 200 && !resp.IsBodyStream() && len(resp.Body()) == 0 {
		return errRequestTimeout
	}
	return nil
}
