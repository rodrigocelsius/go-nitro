// DB BLinq — custom routes (list/get/create/update/remove) with Store + DSL.
// For CRUD via Resource (no handlers): see ../db-blinq
//
//	export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
//	go run .
package main

import (
	"context"
	"log"
	"strconv"

	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	"github.com/rodrigocelsius/go-nitro/examples/v1/productmodel/v2"
	"github.com/rodrigocelsius/go-nitro/examples/v1/productmodel/v2/modelgen"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type productStore struct {
	db *pgxadapter.Store[productmodel.Product]
}

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

	db := pgxadapter.StoreOf[productmodel.Product](nitroDB, "products")
	log.Printf("blinq-handlers: logger=%v entity=%s", db.Gateway().LoggerEnabled(), modelgen.ProductEntity.Name)

	store := &productStore{db: db}
	app := nitro.New()
	app.Get("/products", store.list)
	app.Get("/products/:id", store.get)
	app.Post("/products", store.create)
	app.Post("/products/bulk", store.createMany) // CreateMany + optional WithChunk
	app.Patch("/products/:id", store.update)
	app.Delete("/products/:id", store.remove)

	nitro.Ok(app.Listen(":8080"))
}

func (s *productStore) list(c *nitro.Ctx) error {
	minPrice, _ := strconv.ParseFloat(c.Query("min_price"), 64)
	q := c.Query("q")

	items, err := s.db.Find(c, s.db.Query().
		WhereIf(c.Query("active") == "true", modelgen.Products.Active.Eq(true)).
		WhereIf(c.Query("min_price") != "", modelgen.Products.Price.Gte(minPrice)).
		WhereIf(q != "", modelgen.Products.Name.IContains(q)).
		OrderBy(modelgen.Products.CreatedAt.OrderDesc()).
		Limit(50))
	if err != nil {
		return err
	}
	return c.OK(items)
}

func (s *productStore) get(c *nitro.Ctx) error {
	id := c.Param("id")
	if id == "" {
		return nitro.BadRequest("id required")
	}
	p, err := s.db.Get(c, id)
	if err != nil {
		return err
	}
	return c.OK(p)
}

func (s *productStore) create(c *nitro.Ctx) error {
	p, err := nitro.Parse[productmodel.Product](c)
	if err != nil {
		return err
	}
	created, err := s.db.CreateOne(c, &p)
	if err != nil {
		return err
	}
	return c.Created(created)
}

func (s *productStore) createMany(c *nitro.Ctx) error {
	rows, err := nitro.Parse[[]productmodel.Product](c)
	if err != nil {
		return err
	}
	created, err := s.db.CreateMany(c, rows)
	if err != nil {
		return err
	}
	return c.Created(created)
}

func (s *productStore) update(c *nitro.Ctx) error {
	id := c.Param("id")
	if id == "" {
		return nitro.BadRequest("id required")
	}
	in, err := nitro.Parse[productmodel.Product](c)
	if err != nil {
		return err
	}
	p, err := s.db.Update(c, id,
		modelgen.Products.Name.Set(in.Name),
		modelgen.Products.Price.Set(in.Price),
		modelgen.Products.Active.Set(in.Active),
	)
	if err != nil {
		return err
	}
	return c.OK(p)
}

func (s *productStore) remove(c *nitro.Ctx) error {
	id := c.Param("id")
	if id == "" {
		return nitro.BadRequest("id required")
	}
	if err := s.db.Delete(c, id); err != nil {
		return err
	}
	return c.NoContent()
}
