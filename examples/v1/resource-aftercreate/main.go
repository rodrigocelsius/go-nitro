// AfterCreate — CRUD create + side effect in ResourceConfig.AfterCreate.
//
// Creating an order via POST /orders also creates an open invoice (financial row).
//
//	go run .
//
//	curl -X POST localhost:8080/orders -H 'Content-Type: application/json' \
//	  -d '{"customer_id":"c1","total":99.90}'
package main

import (
	"context"
	"sync"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type Order struct {
	nitro.Model
	CustomerID string  `json:"customer_id" validate:"required"`
	Total      float64 `json:"total" validate:"gt=0"`
	Status     string  `json:"status" default:"draft"`
}

type Invoice struct {
	nitro.Model
	OrderID nitro.UUID `json:"order_id"`
	Amount  float64    `json:"amount"`
	Status  string     `json:"status" default:"open"`
}

var (
	orders   = nitro.NewMemoryRepo[Order]()
	invoices = nitro.NewMemoryRepo[Invoice]()
	invMu    sync.Mutex
)

func main() {
	app := nitro.New(nitro.Config{DisableDefaults: true})

	app.Resource(&Order{}, nitro.AdaptRepo(orders), &nitro.ResourceConfig{
		NoCache: true,
		AfterCreate: func(ctx context.Context, entity any) error {
			o, ok := entity.(*Order)
			if !ok || o == nil {
				return nil
			}
			invMu.Lock()
			defer invMu.Unlock()
			return invoices.Create(ctx, &Invoice{
				OrderID: o.ID,
				Amount:  o.Total,
				Status:  "open",
			})
		},
	})

	app.Get("/invoices/count", func(c *nitro.Ctx) error {
		invMu.Lock()
		n := invoices.Len()
		invMu.Unlock()
		return c.OK(nitro.Map{"count": n})
	})

	app.Get("/", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{
			"hint": "POST /orders creates order + invoice via AfterCreate",
		})
	})

	nitro.Ok(app.Listen(":8080"))
}
