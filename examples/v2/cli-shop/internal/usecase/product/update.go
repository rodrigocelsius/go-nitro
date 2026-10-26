package product

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
)

func Update(ctx context.Context, repo Repository, in *model.Product) (*model.Product, error) {
	if err := repo.Update(ctx, in); err != nil {
		return nil, err
	}
	return in, nil
}
