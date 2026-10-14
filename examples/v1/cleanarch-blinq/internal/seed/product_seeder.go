package seed

import (
	"context"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch-blinq/v2/internal/model"
)

// SeedProduct inserts a demo product (nitrogen seed / cmd/seed).
func SeedProduct(ctx context.Context, db *sqladapter.DB) error {
	store := sqladapter.StoreOf[model.Product](db, "products")
	_, err := store.CreateOne(ctx, &model.Product{
		Name:   "Seeded Widget",
		Price:  19.9,
		Active: true,
	})
	return err
}
