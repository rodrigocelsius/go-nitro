package order

import (
	"context"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/model"
)

// Pending lists pending orders (GET /orders/queries/pending?page=&per_page=).
// repo is bound at registration time: nitro.Q("pending", order.Pending(repo)).
func Pending(repo Repository) func(ctx context.Context, page nitro.Page) ([]model.Order, error) {
	return func(ctx context.Context, page nitro.Page) ([]model.Order, error) {
		res, err := repo.List(ctx, page.Number, page.PerPage)
		if err != nil {
			return nil, err
		}
		out := make([]model.Order, 0, len(res.Items))
		for _, item := range res.Items {
			o, ok := item.(*model.Order)
			if !ok {
				continue
			}
			if o.Status == "pending" {
				out = append(out, *o)
			}
		}
		return out, nil
	}
}
