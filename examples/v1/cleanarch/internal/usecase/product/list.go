package product

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/model"
)

func List(ctx context.Context, repo Repository) ([]model.Product, error) {
	return repo.FindAll(ctx)
}
