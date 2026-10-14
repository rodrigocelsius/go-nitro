// Package ports defines the Clean Architecture ports (interfaces)
// for the Nitro framework. These interfaces are implemented by the
// adapters layer and consumed by the usecases layer.
package ports

import "github.com/rodrigocelsius/go-nitro/domain/v2"

// Router defines HTTP router behavior.
// Concrete implementations live in the adapters layer (e.g. radix tree).
type Router interface {
	// Add registers a route with the given method, path, and handler.
	Add(method, path string, h domain.Handler)

	// Lookup resolves method+path to a Handler, returning true
	// if found. Method and path are []byte to avoid string
	// conversion on the request path (zero-allocation).
	Lookup(method, path []byte, c *domain.Ctx) (domain.Handler, bool)

	// Routes returns all registered routes, for introspection
	// and code generation.
	Routes() []domain.RouteInfo
}
