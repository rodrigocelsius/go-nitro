// Package usecases contains the Nitro framework business logic.
// These functions depend only on the domain layer and do not know
// concrete infrastructure implementations.
package usecases

import "github.com/rodrigocelsius/go-nitro/domain/v2"

// CompileHandler flattens global (pre/post) and route middlewares into a
// single Handler, resolved once at registration time —
// never recomputed per request. Validated by benchmark:
// a pre-resolved pipeline without next() is ~9-12x faster and
// zero-allocation compared to an explicit next() style with
// recursive closures.
func CompileHandler(handler domain.Handler, globalPre, globalPost, routeMws []domain.Handler) domain.Handler {
	// Fast path: with no middleware, return the original handler without
	// any extra layer of indirection.
	if len(globalPre) == 0 && len(globalPost) == 0 && len(routeMws) == 0 {
		return handler
	}

	pre := make([]domain.Handler, 0, len(globalPre)+len(routeMws))
	pre = append(pre, globalPre...)
	pre = append(pre, routeMws...)

	post := make([]domain.Handler, len(globalPost))
	copy(post, globalPost)

	return func(c *domain.Ctx) error {
		for _, h := range pre {
			if err := h(c); err != nil {
				primeStatus(c, err)
				runPost(post, c)
				return err
			}
		}
		err := handler(c)
		if err != nil {
			// Ensures HTTP status before the access log / span end (body may still
			// be written by the adapter ErrorHandler).
			primeStatus(c, err)
		}
		runPost(post, c)
		return err
	}
}

func runPost(post []domain.Handler, c *domain.Ctx) {
	for _, h := range post {
		_ = h(c)
	}
}

func primeStatus(c *domain.Ctx, err error) {
	if httpErr, ok := err.(domain.HTTPError); ok {
		c.SetStatus(httpErr.StatusCode())
		return
	}
	if st := c.Status(); st == 0 || st == 200 {
		c.SetStatus(500)
	}
}
