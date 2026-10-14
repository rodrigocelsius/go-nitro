package sqladapter_test

import (
	"context"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// noTagWidget exercises the fallback naming: CustomerName has no `db`/`json`
// tag at all (defaults to column/key "customer_name"), UnitPriceUSD only
// customizes `db` (json still defaults to "unit_price_usd"), and Active has
// both tags explicit (old style, unaffected).
type noTagWidget struct {
	nitro.Model
	CustomerName string  `validate:"required"`
	UnitPriceUSD float64 `db:"price_usd"`
	Active       bool    `json:"active" db:"active"`
}

const noTagWidgetSchema = `
CREATE TABLE no_tag_widgets (
  id TEXT PRIMARY KEY NOT NULL,
  customer_name TEXT NOT NULL,
  price_usd REAL NOT NULL DEFAULT 0,
  active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);`

func TestStore_CreateUpdate_WithoutDBTag_DefaultsToSnakeCase(t *testing.T) {
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatalf("OpenTest: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Gateway().Exec(ctx, "schema", noTagWidgetSchema, nil); err != nil {
		t.Fatalf("schema: %v", err)
	}

	store := sqladapter.StoreOf[noTagWidget](db, "no_tag_widgets")

	created, err := store.CreateOne(ctx, &noTagWidget{
		CustomerName: "Ada",
		UnitPriceUSD: 9.5,
		Active:       true,
	})
	if err != nil {
		t.Fatalf("CreateOne: %v", err)
	}
	if created.CustomerName != "Ada" {
		t.Fatalf("CustomerName not persisted: %#v", created)
	}
	if created.UnitPriceUSD != 9.5 {
		t.Fatalf("UnitPriceUSD not persisted via db:\"price_usd\": %#v", created)
	}

	got, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CustomerName != "Ada" || got.UnitPriceUSD != 9.5 {
		t.Fatalf("Get mismatch: %#v", got)
	}
}
