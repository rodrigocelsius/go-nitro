//go:build memory

// Memory-only demo (no Postgres).
//
//	go run -tags memory .
package main

import (
	"context"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/memory"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
	"github.com/rodrigocelsius/go-nitro/examples/v1/productmodel/v2/modelgen"
)

func main() {
	p := memory.New("products", []map[string]any{
		{"id": "01", "name": "Keyboard", "price": 99.0, "active": true},
		{"id": "02", "name": "Mouse", "price": 25.0, "active": true},
		{"id": "03", "name": "Monitor", "price": 300.0, "active": false},
	})

	ctx := context.Background()
	var out []map[string]any
	err := query.From(modelgen.ProductEntity).
		Where(modelgen.Products.Price.Gte(50.0).And(field.IsTrue(modelgen.Products.Active))).
		OrderBy(modelgen.Products.Price.OrderDesc()).
		Collect(ctx, p, &out)
	if err != nil {
		panic(err)
	}
	for _, row := range out {
		fmt.Printf("%v — %v\n", row["name"], row["price"])
	}

	plan, _ := query.From(modelgen.ProductEntity).Where(modelgen.Products.ID.Eq("01")).Explain(ctx, p)
	fmt.Println("explain:", plan.Text)
}
