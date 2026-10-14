package product

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/model"
)

// ApplyDiscount reduces product price by percent (0–100).
func ApplyDiscount(ctx context.Context, repo Repository, id string, percent float64) (*model.Product, error) {
	p, err := repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Price = p.Price * (1 - percent/100)
	if err := repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}
