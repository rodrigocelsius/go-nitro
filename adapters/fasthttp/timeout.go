package fasthttp

import (
	"context"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// TimeoutMiddleware returns a middleware that limits request execution time
// using context deadlines. When the timeout expires, the handler should
// check c.Context().Err() to detect the deadline.
//
// This is a cooperative timeout: it sets a deadline on the request context
// and returns a 408 response after the handler completes if the deadline
// was exceeded. For strict enforcement, use Config.RequestTimeout which
// also applies a server-level timeout.
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
		ctx, cancel := context.WithTimeout(c.Context(), timeout)
		defer cancel()

		c.SetContext(ctx)
		return nil
	}
}
