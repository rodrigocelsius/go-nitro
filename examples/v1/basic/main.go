// Basic example — Nitro v1 syntax.
//
// Demonstrates: routing, JSON responses, error handling, groups, middleware.
//
// Run:
//
//	go run main.go
//
// Test:
//
//	curl http://localhost:8080/
//	curl http://localhost:8080/hello/world
//	curl -X POST http://localhost:8080/api/users -d '{"name":"John"}'
//	curl http://localhost:8080/api/users/999
package main

import (
	"context"
	"strings"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
	app := nitro.New()
	app.Prometheus()
	app.CORS().ETag().Compress() // one-liners; ETag before Compress

	api := app.Group("/api")

	api.Get("/ping", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"message": "pong"})
	})

	api.Get("/users/:id", getUser)
	api.Post("/users", createUser)

	app.Get("/hello/:name", func(c *nitro.Ctx) error {
		return c.Text("Hello " + c.Param("name"))
	})

	app.Get("/", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{
			"service": "nitro-v1-basic",
			"status":  "running",
		})
	})

	_ = app.Schedule("1h", func(ctx context.Context) error {
		// example: periodic job coordinated with app shutdown
		return nil
	})

	nitro.Ok(app.Listen(":8080"))
}

func getUser(c *nitro.Ctx) error {
	id := c.Param("id")
	if id == "" {
		return nitro.ErrCode("MISSING_ID", "missing user id")
	}
	if id == "999" {
		return nitro.NotFound("user not found")
	}
	return c.OK(nitro.Map{"id": id, "name": "John Doe"})
}

func createUser(c *nitro.Ctx) error {
	body, err := nitro.Parse[struct {
		Name string `json:"name" validate:"required"`
	}](c)
	if err != nil {
		return err
	}
	body.Name = strings.TrimSpace(body.Name)
	return c.Created(nitro.Map{"id": "1", "name": body.Name})
}
