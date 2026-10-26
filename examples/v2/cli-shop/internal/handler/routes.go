package handler

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
	productuc "github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/usecase/product"
)

func Register(app *nitro.App, repo productuc.Repository) {
	app.Get("/health", Health)
	h := &ProductHandler{repo: repo}
	app.Get("/products", h.List)
	app.Get("/products/:id", h.Get)
	app.Post("/products", h.Create)
	app.Patch("/products/:id", h.Update)
	app.Delete("/products/:id", h.Delete)
}

func RegisterCategory(app *nitro.App, store nitro.AsResourceRepository) {
	app.Resource(&model.Category{}, store, &nitro.ResourceConfig{
		NoCache:    true,
		Filterable: []string{"name"},
		Sortable:   []string{"name", "id", "created_at", "updated_at"},
		FilterRules: []nitro.FilterRule{
			{Param: "q", Field: "name", Op: "icontains"},
		},
	})
}
