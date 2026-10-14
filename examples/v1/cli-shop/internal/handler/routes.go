package handler

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cli-shop/v2/internal/model"
)

// Register mounts health + Resource CRUD for Invoice (in-memory).
func Register(app *nitro.App, repo *nitro.MemoryRepository[model.Invoice]) {
	app.Get("/health", Health)
	app.Resource(&model.Invoice{}, repo, &nitro.ResourceConfig{
		NoCache:    true,
		Filterable: []string{"total", "customer", "tenant_id"},
		Sortable:   []string{"total", "customer", "tenant_id", "id", "created_at", "updated_at"},
		FilterRules: []nitro.FilterRule{
			{Param: "q", Field: "customer", Op: "icontains"},
		},
	})
}

func RegisterProduct(app *nitro.App, repo *nitro.MemoryRepository[model.Product]) {
	app.Resource(&model.Product{}, repo, &nitro.ResourceConfig{
		NoCache:    true,
		Filterable: []string{"name", "price"},
		Sortable:   []string{"name", "price", "id", "created_at", "updated_at"},
		FilterRules: []nitro.FilterRule{
			{Param: "q", Field: "name", Op: "icontains"},
		},
	})
}
