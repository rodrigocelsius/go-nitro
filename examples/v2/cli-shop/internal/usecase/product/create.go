package product

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
)

func Create(ctx context.Context, repo Repository, in *model.Product) (*model.Product, error) {
	return repo.Create(ctx, in)
}
