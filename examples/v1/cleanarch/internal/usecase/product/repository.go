package product

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/model"
)

// Repository is the persistence port for Product usecases.
type Repository interface {
	Create(ctx context.Context, in *model.Product) (*model.Product, error)
	FindByID(ctx context.Context, id string) (*model.Product, error)
	FindAll(ctx context.Context) ([]model.Product, error)
	Update(ctx context.Context, in *model.Product) error
	Delete(ctx context.Context, id string) error
}
