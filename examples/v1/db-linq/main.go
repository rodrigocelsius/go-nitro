// DB BLinq (legacy name db-linq) — Store + Gateway + Resource.
//
//	export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
//	go run .
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
	log.Printf("gateway logger=%v", store.Gateway().LoggerEnabled())

	app := nitro.New()
	// Path /products via TableName(); extra /search uses WhereIf.
	app.Resource(&productmodel.Product{}, store, &nitro.ResourceConfig{
		NoCache:    true,
		Filterable: []string{"name", "active"},
		FilterRules: []nitro.FilterRule{
			{Param: "min_price", Field: "price", Op: "gte"},
			{Param: "q", Field: "name", Op: "icontains"},
		},
		Sortable: []string{"name", "price", "created_at", "id"},
	})

	app.Get("/products/search", func(c *nitro.Ctx) error {
		items, err := store.Find(c, store.Query().
			WhereIf(c.Query("active") == "true", modelgen.Products.Active.Eq(true)).
			WhereIf(c.Query("q") != "", modelgen.Products.Name.IContains(c.Query("q"))).
			OrderBy(modelgen.Products.CreatedAt.OrderDesc()).
			Limit(50))
		if err != nil {
			return err
		}
		return c.OK(items)
	})

	nitro.Ok(app.Listen(":8080"))
}
