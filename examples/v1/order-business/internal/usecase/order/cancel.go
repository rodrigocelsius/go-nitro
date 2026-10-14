package order

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/model"
)

// CancelInput is the Command body for POST /orders/:id/cancel.
type CancelInput struct {
	Reason string `json:"reason" validate:"required"`
}

// Cancel marks the order cancelled (item Command — single table).
// repo is bound at registration time: nitro.Cmd("cancel", order.Cancel(repo)).
func Cancel(repo Repository) func(ctx context.Context, id string, in CancelInput) (*model.Order, error) {
	return func(ctx context.Context, id string, in CancelInput) (*model.Order, error) {
		o, err := repo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		o.Status = "cancelled"
		o.Note = in.Reason
		return o, repo.Update(ctx, o)
	}
}
