package main

import (
	"context"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// recalcSale is the sqladapter.RecalcFunc for Sale/SaleItem: it computes each
// item's Subtotal and sums it into Sale.Total. Runs inside the same Tx as
// every write that touches items (checkout, add item, or update an item) —
// registered once via sqladapter.Related in routes.go.
func recalcSale(sale *Sale, items []SaleItem) {
	sale.Total = nitro.SumBy(items, func(item *SaleItem) float64 {
		item.Subtotal = float64(item.Quantity) * item.UnitPrice
		return item.Subtotal
	})
}

// NotVoided is a global Rule: runs before every Sale Command (void included) —
// a voided sale is closed for business.
func NotVoided(s *Sale) error {
	if s.Status == "voided" {
		return nitro.Conflict("sale is voided")
	}
	return nil
}

// Void marks a sale as voided — item Command: POST /sales/:id/void.
func Void(repo nitro.TypedRepository[Sale]) func(ctx context.Context, id string) (*Sale, error) {
	return func(ctx context.Context, id string) (*Sale, error) {
		sale, err := repo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		sale.Status = "voided"
		if err := repo.Update(ctx, sale); err != nil {
			return nil, err
		}
		return sale, nil
	}
}

// Today lists sales created since midnight (UTC) — collection Query:
// GET /sales/queries/today. Needs a BLinq filter on created_at, so it takes
// db directly instead of the generic TypedRepository.
func Today(db *sqladapter.DB) func(ctx context.Context, page nitro.Page) ([]Sale, error) {
	return func(ctx context.Context, page nitro.Page) ([]Sale, error) {
		store := sqladapter.StoreOf[Sale](db, "sales")
		since := time.Now().UTC().Truncate(24 * time.Hour)
		return store.Find(ctx, store.Query().
			Where(field.Col[any]("sales", "created_at").Gte(since)).
			OrderBy(field.Col[any]("sales", "created_at").OrderDesc()))
	}
}
