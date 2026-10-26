package product

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
)

func List(ctx context.Context, repo Repository, f Filter) ([]model.Product, error) {
	return repo.FindAll(ctx, f)
}
