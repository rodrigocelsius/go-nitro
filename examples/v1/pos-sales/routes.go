package main

import (
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// Routes wires HTTP routes. Keep all routing here — not in main.
//
// sqladapter.Related derives both tables (sales/sale_items), the FK column
// (sale_id) and the child route segment from Sale/SaleItem by convention —
// no StoreOf, no ExtraRoutes written here:
//
//	POST   /sales                          — checkout: creates Sale + its Items, one Tx
//	GET    /sales/:id/sale_items           — list a sale's items
//	POST   /sales/:id/sale_items           — add one item to an existing sale, one Tx
//	PATCH  /sales/:id/sale_items/:childID  — update one item, recalculates Sale.Total, one Tx
func Routes(app *nitro.App, db *sqladapter.DB) {
	app.Get("/health", Health)

	repo := sqladapter.RepositoryFor[Sale](db)

	sqladapter.Related(app, db, recalcSale, &nitro.ResourceConfig{
		NoCache:    true,
		Filterable: []string{"status", "customer_name"},
		Rules:      nitro.Rules(NotVoided),
		Commands:   nitro.Commands(nitro.Cmd("void", Void(repo))),
		Queries:    nitro.Queries(nitro.Q("today", Today(db))),
	})
}
