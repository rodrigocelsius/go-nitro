package order

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/model"
)

// PlaceInput is the Command body for POST /orders/place.
type PlaceInput struct {
	CustomerID string  `json:"customer_id" validate:"required"`
	Total      float64 `json:"total" validate:"gte=0"`
}

// Place creates an Order (sales) and its Invoice (financial) atomically —
// an enterprise write that touches two tables in one request. db is bound
// at registration time: nitro.Cmd("place", order.Place(db)).
func Place(db *sqladapter.DB) func(ctx context.Context, in PlaceInput) (*model.Order, error) {
	return func(ctx context.Context, in PlaceInput) (*model.Order, error) {
		var created *model.Order
		err := db.Tx(ctx, func(tx *sqladapter.DB) error {
			orders := sqladapter.StoreOf[model.Order](tx, "orders")
			invoices := sqladapter.StoreOf[model.Invoice](tx, "invoices")

			o, err := orders.CreateOne(ctx, &model.Order{
				CustomerID: in.CustomerID,
				Total:      in.Total,
				Status:     "draft",
			})
			if err != nil {
				return err
			}
			if _, err := invoices.CreateOne(ctx, &model.Invoice{
				OrderID: o.ID,
				Amount:  o.Total,
				Status:  "open",
			}); err != nil {
				return err
			}
			created = o
			return nil
		})
		return created, err
	}
}

// findInvoiceByOrder is shared by Place callers (approve.go) that need the
// invoice created here.
func findInvoiceByOrder(ctx context.Context, invoices *sqladapter.Store[model.Invoice], orderID any) (*model.Invoice, error) {
	return invoices.FindOne(ctx, invoices.Query().Where(field.Col[any]("invoices", "order_id").Eq(orderID)))
}
