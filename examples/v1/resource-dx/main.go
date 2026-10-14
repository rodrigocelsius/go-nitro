// Resource DX — ExtraRoutes, OnMutate, BindMap, NamedHealth, FilterActive (in-memory).
//
//	go run .
//
//	curl -s localhost:8080/health
//	curl -s -X POST localhost:8080/items -H 'Content-Type: application/json' -d '{"name":"Ada"}'
//	curl -s localhost:8080/items/export
//	curl -s -X POST localhost:8080/echo-map -H 'Content-Type: application/json' -d '{"a":1}'
package main

import (
	"context"
	"sync/atomic"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type Item struct {
	nitro.Model
	Name   string `json:"name" validate:"required,min=2" transform:"trim,titlecase=pt" nitro:"public"`
	Status string `json:"status" default:"active" validate:"omitempty,oneof=active inactive"`
	Secret string `json:"secret,omitempty"` // omitted from Public DTO
}

func main() {
	mutates := atomic.Int64{}
	repo := nitro.NewMemoryRepo[Item]()

	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.HealthCheck(nitro.NamedHealth("memory", func() error { return nil }))

	app.Post("/echo-map", func(c *nitro.Ctx) error {
		m, err := c.BindMap()
		if err != nil {
			return err
		}
		return c.OK(m)
	})

	nitro.Resource(app, "/items", repo, &nitro.ResourceConfig{
		NoCache:     true,
		Filterable:  []string{"status"},
		FilterRules: []nitro.FilterRule{nitro.FilterActive()},
		ExtraRoutes: []nitro.ResourceExtraRoute{
			{Method: "GET", Path: "/export", Handler: func(c *nitro.Ctx) error {
				return c.CSV("items.csv", []string{"id", "name"}, [][]string{{"1", "demo"}})
			}},
		},
		OnMutate: func(ctx context.Context, action string, _ any) error {
			mutates.Add(1)
			_ = ctx
			_ = action
			return nil
		},
	})

	app.Get("/", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{
			"hint":    "POST /items, GET /items/export, POST /echo-map, GET /health",
			"mutates": mutates.Load(),
		})
	})

	nitro.Ok(app.Listen(":8080"))
}
