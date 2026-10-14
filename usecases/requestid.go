package usecases

import "github.com/rodrigocelsius/go-nitro/domain/v2"

// RequestIDFromContext returns the request ID stored in the context.
func RequestIDFromContext(c *domain.Ctx) string {
	return c.RequestID()
}
