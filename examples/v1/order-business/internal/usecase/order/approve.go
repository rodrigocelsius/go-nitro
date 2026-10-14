package order

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/model"
)

// ApproveInput is the Command body for POST /orders/:id/approve.
type ApproveInput struct {
	Reason string `json:"reason" validate:"required,min=4"`
}

// Approve moves the order to approved and its invoice to charged — two
// tables, one transaction. db is bound at registration time:
// nitro.Cmd("approve", order.Approve(db)).When(order.HighValue).Roles("manager").
func Approve(db *sqladapter.DB) func(ctx context.Context, id string, in ApproveInput) (*model.Order, error) {
	return func(ctx context.Context, id string, in ApproveInput) (*model.Order, error) {
		var approved *model.Order
		err := db.Tx(ctx, func(tx *sqladapter.DB) error {
			orders := sqladapter.StoreOf[model.Order](tx, "orders")
			invoices := sqladapter.StoreOf[model.Invoice](tx, "invoices")

			o, err := orders.Get(ctx, id)
			if err != nil {
				return err
			}
			if o.Status != "pending" {
				return nitro.Conflict("order is not pending")
			}

			updated, err := orders.Update(ctx, o.ID,
				field.Col[string]("orders", "status").Set("approved"),
				field.Col[string]("orders", "note").Set(in.Reason),
			)
			if err != nil {
				return err
			}

			inv, err := findInvoiceByOrder(ctx, invoices, o.ID)
			if err != nil {
				return err
			}
			if _, err := invoices.Update(ctx, inv.ID,
				field.Col[string]("invoices", "status").Set("charged"),
			); err != nil {
				return err
			}

			approved = updated
			return nil
		})
		return approved, err
	}
}
