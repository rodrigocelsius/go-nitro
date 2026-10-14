// DB sqlc example — Nitro HTTP + sqlc + Gateway (PostgreSQL / pgx).
//
// Pool Nitro → Gateway → AdaptDBTX → sqlc Queries.
//
//	export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
//	go run .
package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	"github.com/rodrigocelsius/go-nitro/examples/v1/db-sqlc/v2/db"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

//go:embed sql/schema.sql
var schemaFS embed.FS

type productStore struct {
	q *db.Queries
}

func main() {
	ctx := context.Background()
	nitroDB, err := pgxadapter.OpenEnv(ctx)
	if err != nil {
		log.Fatalf("db: %v (set DATABASE_URL)", err)
	}
	defer nitroDB.Close()

	dbtx := nitroDB.DBTX()
	if err := nitroDB.ExecFS(ctx, schemaFS, "sql/schema.sql"); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	q := db.New(dbtx)
	log.Printf("sqlc+gateway: logger=%v pool_ok=%v", nitroDB.Gateway.LoggerEnabled(), nitroDB.Pool.Ping(ctx) == nil)

	store := &productStore{q: q}
	app := nitro.New()
	app.Get("/products", store.list)
	app.Get("/products/:id", store.get)
	app.Post("/products", store.create)
	app.Patch("/products/:id", store.update)
	app.Delete("/products/:id", store.remove)

	nitro.Ok(app.Listen(":8080"))
}

func (s *productStore) list(c *nitro.Ctx) error {
	params := db.ListProductsParams{}
	if c.Query("active") == "true" {
		params.Active = pgtype.Bool{Bool: true, Valid: true}
	}
	if min := c.Query("min_price"); min != "" {
		v, err := strconv.ParseFloat(min, 64)
		if err != nil {
			return nitro.BadRequest("min_price must be a number")
		}
		params.MinPrice = pgtype.Float8{Float64: v, Valid: true}
	}
	if name := c.Query("q"); name != "" {
		params.Name = pgtype.Text{String: name, Valid: true}
	}

	items, err := s.q.ListProducts(c.RequestContext(), params)
	if err != nil {
		return err
	}
	return c.OK(items)
}

func (s *productStore) get(c *nitro.Ctx) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return nitro.BadRequest("invalid id")
	}
	p, err := s.q.GetProduct(c.RequestContext(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nitro.NotFound("product not found")
	}
	if err != nil {
		return err
	}
	return c.OK(p)
}

func (s *productStore) create(c *nitro.Ctx) error {
	in, err := nitro.Parse[struct {
		Name   string  `json:"name" validate:"required,min=2"`
		Price  float64 `json:"price" validate:"gte=0"`
		Active *bool   `json:"active"`
	}](c)
	if err != nil {
		return err
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	p, err := s.q.CreateProduct(c.RequestContext(), db.CreateProductParams{
		Name: in.Name, Price: in.Price, Active: active,
	})
	if err != nil {
		return err
	}
	return c.Created(p)
}

func (s *productStore) update(c *nitro.Ctx) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return nitro.BadRequest("invalid id")
	}
	in, err := nitro.Parse[struct {
		Name   string  `json:"name" validate:"required,min=2"`
		Price  float64 `json:"price" validate:"gte=0"`
		Active bool    `json:"active"`
	}](c)
	if err != nil {
		return err
	}
	p, err := s.q.UpdateProduct(c.RequestContext(), db.UpdateProductParams{
		ID: id, Name: in.Name, Price: in.Price, Active: in.Active,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nitro.NotFound("product not found")
	}
	if err != nil {
		return err
	}
	return c.OK(p)
}

func (s *productStore) remove(c *nitro.Ctx) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return nitro.BadRequest("invalid id")
	}
	n, err := s.q.DeleteProduct(c.RequestContext(), id)
	if err != nil {
		return err
	}
	if n == 0 {
		return nitro.NotFound("product not found")
	}
	return c.NoContent()
}
