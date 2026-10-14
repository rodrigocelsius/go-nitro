package repository

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/model"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type ProductRepository struct {
	inner *nitro.MemoryRepository[model.Product]
}

func NewProductMemory() *ProductRepository {
	return &ProductRepository{inner: nitro.NewMemoryRepo[model.Product]()}
}

func (r *ProductRepository) Create(ctx context.Context, in *model.Product) (*model.Product, error) {
	if err := r.inner.Create(ctx, in); err != nil {
		return nil, err
	}
	return in, nil
}

func (r *ProductRepository) FindByID(ctx context.Context, id string) (*model.Product, error) {
	return r.inner.FindByID(ctx, id)
}

func (r *ProductRepository) FindAll(ctx context.Context) ([]model.Product, error) {
	res, err := r.inner.List(ctx, 1, 1000)
	if err != nil {
		return nil, err
	}
	out := make([]model.Product, 0, len(res.Items))
	for _, it := range res.Items {
		if p, ok := it.(*model.Product); ok {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (r *ProductRepository) Update(ctx context.Context, in *model.Product) error {
	return r.inner.Update(ctx, in)
}

func (r *ProductRepository) Delete(ctx context.Context, id string) error {
	return r.inner.Delete(ctx, id)
}
