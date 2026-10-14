// POS sales example — a Sale header with dependent SaleItem line items via
// sqladapter.Related: atomic checkout (Sale + Items in one Tx), item listing,
// adding an item to an existing sale, and item updates that recalculate the
// sale's total — all without ExtraRoutes or hand-written Commands for CRUD.
//
// Run:
//
//	export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
//	go run .
//
// See README.md for the full request walkthrough.
package main

import (
	"context"
	"log"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
	ctx := context.Background()
	db, err := sqladapter.OpenEnv(ctx)
	if err != nil {
		log.Fatalf("db: %v (set DATABASE_URL)", err)
	}
	defer db.Close()
	if err := db.MigrateUp(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	app := nitro.New()
	Routes(app, db)
	nitro.Ok(app.Listen(":8080"))
}
