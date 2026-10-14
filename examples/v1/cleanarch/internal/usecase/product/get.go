package product

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/model"
)

func Get(ctx context.Context, repo Repository, id string) (*model.Product, error) {
	return repo.FindByID(ctx, id)
}
