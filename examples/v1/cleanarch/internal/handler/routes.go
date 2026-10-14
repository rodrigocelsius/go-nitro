package handler

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/repository"
)

// Register wires HTTP routes. Keep all routing here — not in main.
func Register(app *nitro.App, repo *repository.ProductRepository) {
	app.Get("/health", Health)

	h := &ProductHandler{repo: repo}
	app.Get("/products", h.List)
	app.Get("/products/:id", h.Get)
	app.Post("/products", h.Create)
	app.Patch("/products/:id", h.Update)
	app.Delete("/products/:id", h.Delete)
	app.Post("/products/:id/discount", h.Discount)
}
