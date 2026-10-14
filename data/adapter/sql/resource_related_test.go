package sqladapter_test

import (
	"context"
	"encoding/json"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// RelatedSale/RelatedSaleItem exercise sqladapter.Related: a header (Sale)
// with dependent line items (SaleItem), atomic nested create, child listing,
// adding a child to an existing parent, and child update with recalculation.
type RelatedSale struct {
	nitro.Model
	CustomerName string            `json:"customer_name" db:"customer_name"`
	Total        float64           `json:"total" db:"total"`
	Items        []RelatedSaleItem `json:"items" db:"-"`
}

type RelatedSaleItem struct {
	nitro.Model
	SaleID    nitro.UUID `json:"sale_id" db:"related_sale_id"`
	SKU       string     `json:"sku" db:"sku" validate:"required"`
	Quantity  int        `json:"quantity" db:"quantity" validate:"required,gt=0"`
	UnitPrice float64    `json:"unit_price" db:"unit_price" validate:"required,gt=0"`
	Subtotal  float64    `json:"subtotal" db:"subtotal"`
}

const relatedSchemaSQLite = `
CREATE TABLE related_sales (
  id TEXT PRIMARY KEY NOT NULL,
  customer_name TEXT NOT NULL,
  total REAL NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);
CREATE TABLE related_sale_items (
  id TEXT PRIMARY KEY NOT NULL,
  related_sale_id TEXT NOT NULL,
  sku TEXT NOT NULL UNIQUE,
  quantity INTEGER NOT NULL,
  unit_price REAL NOT NULL,
  subtotal REAL NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);`

func relatedRecalc(sale *RelatedSale, items []RelatedSaleItem) {
	sale.Total = nitro.SumBy(items, func(item *RelatedSaleItem) float64 {
		item.Subtotal = float64(item.Quantity) * item.UnitPrice
		return item.Subtotal
	})
}

func relatedTestApp(t *testing.T) (*nitro.App, *sqladapter.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatalf("OpenTest: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Gateway().Exec(ctx, "schema", relatedSchemaSQLite, nil); err != nil {
		t.Fatalf("schema: %v", err)
	}

	app := nitro.New(nitro.Config{DisableDefaults: true})
	sqladapter.Related(app, db, RecalcFunc(relatedRecalc), &nitro.ResourceConfig{NoCache: true})
	return app, db
}

// RecalcFunc avoids the caller needing to spell out sqladapter.RecalcFunc[RelatedSale, RelatedSaleItem]{...}.
type RecalcFunc = sqladapter.RecalcFunc[RelatedSale, RelatedSaleItem]

func TestRelated_CreateAtomicWithChildren(t *testing.T) {
	app, _ := relatedTestApp(t)
	tc := nitro.NewTestClient(app)

	resp := tc.Post(t, "/related_sales", `{
		"customer_name": "Ada",
		"items": [
			{"sku": "A1", "quantity": 2, "unit_price": 10},
			{"sku": "A2", "quantity": 1, "unit_price": 5}
		]
	}`)
	tc.AssertStatus(t, resp, 201)
	tc.AssertBodyContains(t, resp, `"total":25`)

	id := extractRelatedID(t, resp.Body())

	list := tc.Get(t, "/related_sales/"+id+"/related_sale_items")
	tc.AssertStatus(t, list, 200)
	tc.AssertBodyContains(t, list, `"sku":"A1"`)
	tc.AssertBodyContains(t, list, `"sku":"A2"`)
}

func TestRelated_CreateRollsBackOnChildFailure(t *testing.T) {
	app, db := relatedTestApp(t)
	tc := nitro.NewTestClient(app)

	// Both items share the same SKU — the UNIQUE constraint fails on the
	// second insert, so the whole Tx (parent + both children) rolls back.
	resp := tc.Post(t, "/related_sales", `{
		"customer_name": "Rollback Co",
		"items": [
			{"sku": "DUP", "quantity": 1, "unit_price": 10},
			{"sku": "DUP", "quantity": 1, "unit_price": 10}
		]
	}`)
	if resp.StatusCode() < 400 {
		t.Fatalf("expected failure, got %d: %s", resp.StatusCode(), resp.Body())
	}

	store := sqladapter.StoreOf[RelatedSale](db, "related_sales")
	rows, err := store.Find(context.Background(), store.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected rollback, got %d sales", len(rows))
	}
}

func TestRelated_AddChildToExistingParent(t *testing.T) {
	app, _ := relatedTestApp(t)
	tc := nitro.NewTestClient(app)

	created := tc.Post(t, "/related_sales", `{"customer_name": "Bob", "items": [{"sku": "B1", "quantity": 1, "unit_price": 10}]}`)
	tc.AssertStatus(t, created, 201)
	id := extractRelatedID(t, created.Body())

	added := tc.Post(t, "/related_sales/"+id+"/related_sale_items", `{"sku": "B2", "quantity": 2, "unit_price": 20}`)
	tc.AssertStatus(t, added, 201)
	tc.AssertBodyContains(t, added, `"sku":"B2"`)

	got := tc.Get(t, "/related_sales/"+id)
	tc.AssertStatus(t, got, 200)
	tc.AssertBodyContains(t, got, `"total":50`) // 10 (existing) + 40 (2*20)
}

func TestRelated_UpdateChildRecalculatesParent(t *testing.T) {
	app, _ := relatedTestApp(t)
	tc := nitro.NewTestClient(app)

	created := tc.Post(t, "/related_sales", `{"customer_name": "Cid", "items": [{"sku": "C1", "quantity": 1, "unit_price": 10}]}`)
	tc.AssertStatus(t, created, 201)
	id := extractRelatedID(t, created.Body())

	list := tc.Get(t, "/related_sales/"+id+"/related_sale_items")
	tc.AssertStatus(t, list, 200)
	childID := extractFirstRelatedItemID(t, list.Body())

	patched := tc.Patch(t, "/related_sales/"+id+"/related_sale_items/"+childID, `{"quantity": 5}`)
	tc.AssertStatus(t, patched, 200)

	got := tc.Get(t, "/related_sales/"+id)
	tc.AssertStatus(t, got, 200)
	tc.AssertBodyContains(t, got, `"total":50`) // 5*10
}

func TestRelated_PanicsWithoutNestedSliceField(t *testing.T) {
	type Header struct {
		nitro.Model
		Name string `db:"name"`
	}
	type Line struct {
		nitro.Model
		HeaderID nitro.UUID `db:"header_id"`
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic: Header has no []Line field")
		}
	}()
	ctx := context.Background()
	db, _ := sqladapter.OpenTest(ctx)
	defer db.Close()
	app := nitro.New(nitro.Config{DisableDefaults: true})
	sqladapter.Related(app, db, sqladapter.RecalcFunc[Header, Line](func(*Header, []Line) {}), &nitro.ResourceConfig{})
}

func TestRelated_PanicsWithoutFKColumn(t *testing.T) {
	type Line struct {
		nitro.Model
		// no field with db:"header_id" — must panic at registration.
		Label string `db:"label"`
	}
	type Header struct {
		nitro.Model
		Name  string `db:"name"`
		Lines []Line `json:"lines" db:"-"`
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic: Line has no FK field to Header")
		}
	}()
	ctx := context.Background()
	db, _ := sqladapter.OpenTest(ctx)
	defer db.Close()
	app := nitro.New(nitro.Config{DisableDefaults: true})
	sqladapter.Related(app, db, sqladapter.RecalcFunc[Header, Line](func(*Header, []Line) {}), &nitro.ResourceConfig{})
}

func extractRelatedID(t *testing.T, body []byte) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	id, _ := m["id"].(string)
	if id == "" {
		t.Fatalf("missing id: %s", body)
	}
	return id
}

func extractFirstRelatedItemID(t *testing.T, body []byte) string {
	t.Helper()
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatalf("expected at least one item: %s", body)
	}
	id, _ := items[0]["id"].(string)
	if id == "" {
		t.Fatalf("missing id: %s", body)
	}
	return id
}
