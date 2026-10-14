//go:build !memory

// DB BLinq — Store + Gateway + Resource (CRUD without hand-written handlers).
//
//	export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
//	go run .
//
//	curl 'http://localhost:8080/products?active=true&min_price=100&q=key&sort=-price'
//
// Demo without Postgres: go run -tags memory .
package main

import (
	"context"
	"log"

	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	"github.com/rodrigocelsius/go-nitro/examples/v1/productmodel/v2"
	"github.com/rodrigocelsius/go-nitro/examples/v1/productmodel/v2/modelgen"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
	ctx := context.Background()
	nitroDB, err := pgxadapter.OpenEnv(ctx)
	if err != nil {
		log.Fatalf("db: %v (set DATABASE_URL)", err)
	}
	defer nitroDB.Close()

	if err := nitroDB.MigrateUp(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	store := pgxadapter.StoreOf[productmodel.Product](nitroDB, "products")
	log.Printf("blinq+gateway: logger=%v entity=%s", store.Gateway().LoggerEnabled(), modelgen.ProductEntity.Name)

	app := nitro.New()
	// Path /products via TableName(); Filterable/Sortable inferred from blinq:"index".
	app.Resource(&productmodel.Product{}, store, &nitro.ResourceConfig{
		NoCache: true,
		FilterRules: []nitro.FilterRule{
			{Param: "min_price", Field: "price", Op: "gte"},
			{Param: "q", Field: "name", Op: "icontains"},
		},
	})
	nitro.Ok(app.Listen(":8080"))
}
