// DB sqlx example — Nitro HTTP + sqlx + Gateway (PostgreSQL).
//
// Pool Nitro → Gateway → OpenSQL → sqlx.NewDb (observed queries).
//
//	export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
//	go run .
package main

import (
	"context"
	"embed"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

//go:embed schema.sql
var schemaFS embed.FS

type Product struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name" validate:"required,min=2"`
	Price     float64   `json:"price" db:"price" validate:"gte=0"`
	Active    bool      `json:"active" db:"active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type productStore struct {
	db *sqlx.DB
}

func main() {
	ctx := context.Background()
	nitroDB, err := pgxadapter.OpenEnv(ctx)
	if err != nil {
		log.Fatalf("db: %v (set DATABASE_URL and apply schema.sql)", err)
	}
	defer nitroDB.Close()

	db := sqlx.NewDb(nitroDB.SQL(), "pgx")

	if err := nitroDB.ExecFS(ctx, schemaFS, "schema.sql"); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Printf("sqlx+gateway: logger=%v pool_ok=%v", nitroDB.Gateway.LoggerEnabled(), nitroDB.Pool.Ping(ctx) == nil)

	store := &productStore{db: db}
	app := nitro.New()
	app.Get("/products", store.list)
	app.Get("/products/:id", store.get)
	app.Post("/products", store.create)
	app.Patch("/products/:id", store.update)
	app.Delete("/products/:id", store.remove)

	nitro.Ok(app.Listen(":8080"))
}

func (s *productStore) list(c *nitro.Ctx) error {
	var (
		conds []string
		args  []any
	)
	if c.Query("active") == "true" {
		conds = append(conds, "active = $"+strconv.Itoa(len(args)+1))
		args = append(args, true)
	}
	if min := c.Query("min_price"); min != "" {
		v, err := strconv.ParseFloat(min, 64)
		if err != nil {
			return nitro.BadRequest("min_price must be a number")
		}
		conds = append(conds, "price >= $"+strconv.Itoa(len(args)+1))
		args = append(args, v)
	}
	if name := c.Query("q"); name != "" {
		conds = append(conds, "name ILIKE $"+strconv.Itoa(len(args)+1))
		args = append(args, "%"+name+"%")
	}

	sql := `SELECT id, name, price, active, created_at FROM products`
	if len(conds) > 0 {
		sql += " WHERE " + strings.Join(conds, " AND ")
	}
	sql += " ORDER BY created_at DESC LIMIT 50"

	var items []Product
	if err := s.db.SelectContext(c.RequestContext(), &items, sql, args...); err != nil {
		return err
	}
	if items == nil {
		items = []Product{}
	}
	return c.OK(items)
}

func (s *productStore) get(c *nitro.Ctx) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return nitro.BadRequest("invalid id")
	}
	var p Product
	err = s.db.GetContext(c.RequestContext(), &p,
		`SELECT id, name, price, active, created_at FROM products WHERE id = $1`, id)
	if err != nil {
		return nitro.NotFound("product not found")
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
	var p Product
	err = s.db.GetContext(c.RequestContext(), &p, `
		INSERT INTO products (name, price, active)
		VALUES ($1, $2, $3)
		RETURNING id, name, price, active, created_at`,
		in.Name, in.Price, active)
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
	res, err := s.db.ExecContext(c.RequestContext(), `
		UPDATE products SET name = $1, price = $2, active = $3 WHERE id = $4`,
		in.Name, in.Price, in.Active, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nitro.NotFound("product not found")
	}
	return s.get(c)
}

func (s *productStore) remove(c *nitro.Ctx) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return nitro.BadRequest("invalid id")
	}
	res, err := s.db.ExecContext(c.RequestContext(), `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nitro.NotFound("product not found")
	}
	return c.NoContent()
}
