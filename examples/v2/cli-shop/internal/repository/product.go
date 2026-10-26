package repository

import (
	"context"
	"strings"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/modelgen"
	productuc "github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/usecase/product"
)

type ProductRepository struct {
	store *sqladapter.Store[model.Product]
}

func NewProduct(store *sqladapter.Store[model.Product]) *ProductRepository {
	return &ProductRepository{store: store}
}

var _ productuc.Repository = (*ProductRepository)(nil)

func (r *ProductRepository) Create(ctx context.Context, in *model.Product) (*model.Product, error) {
	return r.store.CreateOne(ctx, in)
}

func (r *ProductRepository) FindByID(ctx context.Context, id string) (*model.Product, error) {
	return r.store.Get(ctx, id)
}

func (r *ProductRepository) FindAll(ctx context.Context, f productuc.Filter) ([]model.Product, error) {
	page, per := f.Page, f.PerPage
	if page < 1 {
		page = 1
	}
	if per < 1 {
		per = 20
	}
	q := r.store.Query()
	q = q.WhereIf(f.Q != "", modelgen.Products.Name.IContains(f.Q))
	switch strings.TrimPrefix(f.Sort, "+") {
	case "name", "Name":
		if strings.HasPrefix(f.Sort, "-") {
			q = q.OrderBy(modelgen.Products.Name.OrderDesc())
		} else {
			q = q.OrderBy(modelgen.Products.Name.OrderAsc())
		}
	case "-name":
		q = q.OrderBy(modelgen.Products.Name.OrderDesc())
	case "price", "Price":
		if strings.HasPrefix(f.Sort, "-") {
			q = q.OrderBy(modelgen.Products.Price.OrderDesc())
		} else {
			q = q.OrderBy(modelgen.Products.Price.OrderAsc())
		}
	case "-price":
		q = q.OrderBy(modelgen.Products.Price.OrderDesc())
	case "stock", "Stock":
		if strings.HasPrefix(f.Sort, "-") {
			q = q.OrderBy(modelgen.Products.Stock.OrderDesc())
		} else {
			q = q.OrderBy(modelgen.Products.Stock.OrderAsc())
		}
	case "-stock":
		q = q.OrderBy(modelgen.Products.Stock.OrderDesc())
	default:
		q = q.OrderBy(modelgen.Products.CreatedAt.OrderDesc())
	}
	q = q.Paginate(uint64(page), uint64(per))
	return r.store.Find(ctx, q)
}

func (r *ProductRepository) Update(ctx context.Context, in *model.Product) error {
	updated, err := r.store.Update(ctx, in.ID,
		modelgen.Products.Name.Set(in.Name),
		modelgen.Products.Price.Set(in.Price),
		modelgen.Products.Stock.Set(in.Stock),
	)
	if err != nil {
		return err
	}
	*in = *updated
	return nil
}

func (r *ProductRepository) Delete(ctx context.Context, id string) error {
	return r.store.Delete(ctx, id)
}
