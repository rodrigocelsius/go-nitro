package product

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
)

type Filter struct {
	Q       string
	Sort    string // e.g. name, -price, created_at (default -created_at)
	Page    int
	PerPage int
}

// Repository is the persistence port for Product usecases.
type Repository interface {
	Create(ctx context.Context, in *model.Product) (*model.Product, error)
	FindByID(ctx context.Context, id string) (*model.Product, error)
	FindAll(ctx context.Context, f Filter) ([]model.Product, error)
	Update(ctx context.Context, in *model.Product) error
	Delete(ctx context.Context, id string) error
}
