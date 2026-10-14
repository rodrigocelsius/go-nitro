package main

import (
	"context"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func TestAfterCreate_CreatesInvoice(t *testing.T) {
	orders = nitro.NewMemoryRepo[Order]()
	invoices = nitro.NewMemoryRepo[Invoice]()

	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource(&Order{}, nitro.AdaptRepo(orders), &nitro.ResourceConfig{
		NoCache: true,
		AfterCreate: func(ctx context.Context, entity any) error {
			o := entity.(*Order)
			return invoices.Create(ctx, &Invoice{
				OrderID: o.ID,
				Amount:  o.Total,
				Status:  "open",
			})
		},
	})
	app.Get("/invoices/count", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"count": invoices.Len()})
	})

	tc := nitro.NewTestClient(app)
	create := tc.PostJSON(t, "/orders", map[string]any{
		"customer_id": "c1",
		"total":       42.5,
	})
	tc.AssertStatus(t, create, 201)

	count := tc.Get(t, "/invoices/count")
	tc.AssertStatus(t, count, 200)
	tc.AssertBodyContains(t, count, `"count":1`)
}
