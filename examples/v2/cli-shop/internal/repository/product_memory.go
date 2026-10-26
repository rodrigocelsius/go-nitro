package repository

import (
	"context"
	"strings"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
	productuc "github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/usecase/product"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type ProductMemory struct {
	inner *nitro.MemoryRepository[model.Product]
}

func NewProductMemory() *ProductMemory {
	return &ProductMemory{inner: nitro.NewMemoryRepo[model.Product]()}
}

var _ productuc.Repository = (*ProductMemory)(nil)

func (r *ProductMemory) Create(ctx context.Context, in *model.Product) (*model.Product, error) {
	if err := r.inner.Create(ctx, in); err != nil {
		return nil, err
	}
	return in, nil
}

func (r *ProductMemory) FindByID(ctx context.Context, id string) (*model.Product, error) {
	return r.inner.FindByID(ctx, id)
}

func (r *ProductMemory) FindAll(ctx context.Context, f productuc.Filter) ([]model.Product, error) {
	res, err := r.inner.List(ctx, 1, 10000)
	if err != nil {
		return nil, err
	}
	out := make([]model.Product, 0, len(res.Items))
	q := strings.ToLower(f.Q)
	for _, it := range res.Items {
		p, ok := it.(*model.Product)
		if !ok {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(p.Name), q) {
			continue
		}
		out = append(out, *p)
	}
	page, per := f.Page, f.PerPage
	if page < 1 {
		page = 1
	}
	if per < 1 {
		per = 20
	}
	start := (page - 1) * per
	if start > len(out) {
		start = len(out)
	}
	end := start + per
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], nil
}

func (r *ProductMemory) Update(ctx context.Context, in *model.Product) error {
	return r.inner.Update(ctx, in)
}

func (r *ProductMemory) Delete(ctx context.Context, id string) error {
	return r.inner.Delete(ctx, id)
}
