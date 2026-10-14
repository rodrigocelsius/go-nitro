package order

import (
	"context"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/model"
)

// Submit moves draft → pending (item Command, no body — single table).
// repo is bound at registration time: nitro.Cmd("submit", order.Submit(repo)).
func Submit(repo Repository) func(ctx context.Context, id string) (*model.Order, error) {
	return func(ctx context.Context, id string) (*model.Order, error) {
		o, err := repo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if o.Status != "draft" {
			return nil, nitro.Conflict("order is not draft")
		}
		if o.Total <= 0 {
			return nil, nitro.BadRequest("total must be positive")
		}
		o.Status = "pending"
		return o, repo.Update(ctx, o)
	}
}
