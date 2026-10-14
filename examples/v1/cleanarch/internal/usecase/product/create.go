package product

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/model"
)

// Create persists a new Product.
func Create(ctx context.Context, repo Repository, in *model.Product) (*model.Product, error) {
	return repo.Create(ctx, in)
}
