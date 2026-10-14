// Conventions example — everything below works with zero configuration.
//
// Demonstrates: typed handlers (path/query/body binding by field name),
// chainable app middleware (CORS/ETag/Compress/ResponseTime), per-route
// auth helpers (BasicAuth / APIKeyAuth), Skip, and the Resource conventions
// (plural path, snake_case wire names, auto tenant scope).
//
// Run (NITRO_ENV=development enables permissive CORS and verbose errors):
//
//	NITRO_ENV=development go run .
//
// Try:
//
//	curl localhost:8080/products
//	curl -X POST localhost:8080/products -d '{"name":"Pen","unit_price":2.5}'
//	curl localhost:8080/greet/Ada?shout=true
//	curl -u admin:secret localhost:8080/admin/stats
//	curl -H 'X-API-Key: dev-key' localhost:8080/internal/ping
package main

import (
	"context"
	"strings"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// Product is served at /products (plural of the model) with snake_case JSON.
type Product struct {
	nitro.Model
	Name      string  `validate:"required,min=2"` // wire name: "name"
	UnitPrice float64 `validate:"gte=0"`          // wire name: "unit_price"
}

// GreetIn is bound by convention: ":name" → Name, "?shout=" → Shout.
type GreetIn struct {
	Name  string `validate:"required"`
	Shout bool
}

type GreetOut struct {
	Message string `json:"message"`
}

func greet(_ context.Context, in *GreetIn) (*GreetOut, error) {
	msg := "Hello, " + in.Name
	if in.Shout {
		msg = strings.ToUpper(msg) + "!"
	}
	return &GreetOut{Message: msg}, nil
}

func buildApp() *nitro.App {
	app := nitro.New()
	app.CORS().ETag().Compress().ResponseTime() // ETag before Compress

	app.Resource(&Product{}, nitro.NewMemoryRepo[Product](), &nitro.ResourceConfig{Public: true})
	app.Get("/greet/:name", nitro.Handle(greet))

	app.Get("/admin/stats", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"user": c.Get("user")})
	}, nitro.BasicAuth(map[string]string{"admin": "secret"}))

	internal := app.Group("/internal")
	internal.Use(nitro.APIKeyAuth("dev-key"))
	internal.Get("/ping", func(c *nitro.Ctx) error { return c.Text("pong") })

	// Skip: run an expensive middleware everywhere except /health.
	app.Get("/health", func(c *nitro.Ctx) error { return c.OK(nitro.Map{"status": "ok"}) },
		nitro.Skip(func(c *nitro.Ctx) bool { return true }, nitro.APIKeyAuth("never-used")))
	return app
}

func main() {
	nitro.Ok(buildApp().Listen(":8080"))
}
