package ports

import "github.com/rodrigocelsius/go-nitro/domain/v2"

// RequestHandler is the signature of a handler that processes a request.
// Used by Server to expose the compiled handler.
type RequestHandler func(c *domain.Ctx) error

// Server defines HTTP server behavior.
// Concrete implementations live in the adapters layer (e.g. fasthttp).
type Server interface {
	// Listen starts the HTTP server on the given address.
	// Blocks until the server is shut down.
	Listen(addr string) error

	// Shutdown shuts down the server gracefully.
	Shutdown() error

	// Handler returns the function that processes requests.
	// Can be used for in-process tests or composition with
	// other servers.
	Handler() RequestHandler
}
