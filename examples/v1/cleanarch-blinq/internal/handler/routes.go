package handler

import (
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch-blinq/v2/internal/model"
)

// Register mounts health + Resource CRUD for Product (Convention over Configuration).
func Register(app *nitro.App, store *sqladapter.Store[model.Product]) {
	app.Get("/health", Health)
	app.Resource(&model.Product{}, store, &nitro.ResourceConfig{
		NoCache:    true,
		Filterable: []string{"name", "price", "active"},
		Sortable:   []string{"name", "price", "active", "id", "created_at", "updated_at"},
		FilterRules: []nitro.FilterRule{
			{Param: "q", Field: "name", Op: "icontains"},
		},
	})
}
