package handler

import (
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/model"
	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/repository"
	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/usecase/order"
)

// Register mounts health + Resource CRUD + business Commands/Queries for
// Order. mw runs on every /orders route (auth in production, a role stub in
// tests) — see cmd/server/main.go and main_test.go.
func Register(app *nitro.App, db *sqladapter.DB, store *sqladapter.Store[model.Order], mw ...nitro.Handler) {
	app.Get("/health", Health)

	repo := repository.NewOrderRepository(store)

	app.Resource(&model.Order{}, store, &nitro.ResourceConfig{
		NoCache:    true,
		Middleware: mw,
		Filterable: []string{"status", "customer_id"},
		Sortable:   []string{"status", "total", "created_at"},

		// Runs before every item Command (Approve, Cancel).
		Rules: nitro.Rules(order.NotCancelled),

		Commands: nitro.Commands(
			// place: collection Command → POST /orders/place (insert order + invoice, one tx).
			nitro.Cmd("place", order.Place(db)),
			// submit: item Command, no body → POST /orders/:id/submit.
			nitro.Cmd("submit", order.Submit(repo)),
			// approve: item Command → POST /orders/:id/approve (update order + invoice, one tx).
			// Extra "manager" role only when the order is high value.
			nitro.Cmd("approve", order.Approve(db)).When(order.HighValue).Roles("manager"),
			// cancel: item Command → POST /orders/:id/cancel.
			nitro.Cmd("cancel", order.Cancel(repo)),
		),

		Queries: nitro.Queries(
			// GET /orders/queries/pending?page=&per_page=
			nitro.Q("pending", order.Pending(repo)),
		),
	})
}
