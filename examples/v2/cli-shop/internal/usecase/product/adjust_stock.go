package product

import (
	"context"
	"errors"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/model"
)

// ErrInsufficientStock is returned when an adjustment would make stock negative.
var ErrInsufficientStock = errors.New("insufficient stock")

// UnitOfWork is the transaction port. *sqladapter.DB satisfies it; every
// repository call made with the ctx passed to fn joins the transaction, so
// the use case stays free of infrastructure types.
type UnitOfWork interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// AdjustStock reads, checks and writes the product in one transaction: two
// concurrent adjustments cannot both pass the check with a stale read.
func AdjustStock(ctx context.Context, uow UnitOfWork, repo Repository, id string, delta int64) (*model.Product, error) {
	var out *model.Product
	err := uow.InTx(ctx, func(ctx context.Context) error {
		p, err := repo.FindByID(ctx, id)
		if err != nil {
			return err
		}
		if p.Stock+delta < 0 {
			return ErrInsufficientStock // rolls back
		}
		p.Stock += delta
		if err := repo.Update(ctx, p); err != nil {
			return err
		}
		out = p
		return nil
	})
	return out, err
}
