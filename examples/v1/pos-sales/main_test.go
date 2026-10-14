package main

import (
	"context"
	"encoding/json"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

const schemaSQLite = `
CREATE TABLE IF NOT EXISTS sales (
    id            TEXT PRIMARY KEY NOT NULL,
    customer_name TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'open',
    total         REAL NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME
);
CREATE TABLE IF NOT EXISTS sale_items (
    id            TEXT PRIMARY KEY NOT NULL,
    sale_id       TEXT NOT NULL,
    sku           TEXT NOT NULL,
    quantity      INTEGER NOT NULL,
    unit_price    REAL NOT NULL,
    subtotal      REAL NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME
);`

// testApp wires the same Routes(...) used in production, against an isolated
// in-memory SQLite DB (sqladapter.OpenTest) — no Docker needed.
func testApp(t *testing.T) (*nitro.App, *sqladapter.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatalf("OpenTest: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Gateway().Exec(ctx, "schema", schemaSQLite, nil); err != nil {
		t.Fatalf("schema: %v", err)
	}

	app := nitro.New()
	Routes(app, db)
	return app, db
}

func extractField(t *testing.T, body []byte, key string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	v, _ := m[key].(string)
	if v == "" {
		t.Fatalf("missing %s: %s", key, body)
	}
	return v
}

func extractFirstItemID(t *testing.T, body []byte) string {
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

func checkout(t *testing.T, tc *nitro.TestClient, body string) string {
	t.Helper()
	resp := tc.Post(t, "/sales", body)
	tc.AssertStatus(t, resp, 201)
	return extractField(t, resp.Body(), "id")
}

func TestCheckout_CreatesSaleAndItemsWithTotal(t *testing.T) {
	app, _ := testApp(t)
	tc := nitro.NewTestClient(app)

	id := checkout(t, tc, `{
		"customer_name": "Ada",
		"items": [
			{"sku": "COFFEE", "quantity": 2, "unit_price": 8.5},
			{"sku": "CROISSANT", "quantity": 1, "unit_price": 6}
		]
	}`)

	got := tc.Get(t, "/sales/"+id)
	tc.AssertStatus(t, got, 200)
	tc.AssertBodyContains(t, got, `"total":23`)

	list := tc.Get(t, "/sales/"+id+"/sale_items")
	tc.AssertStatus(t, list, 200)
	tc.AssertBodyContains(t, list, `"sku":"COFFEE"`)
	tc.AssertBodyContains(t, list, `"sku":"CROISSANT"`)
}

func TestCheckout_RollsBackWhenAnItemIsInvalid(t *testing.T) {
	app, db := testApp(t)
	tc := nitro.NewTestClient(app)

	// quantity 0 fails SaleItem's validate:"required,gt=0" during Parse[SaleItem]
	// dive — nothing is persisted, not even the Sale header.
	resp := tc.Post(t, "/sales", `{
		"customer_name": "Bob",
		"items": [{"sku": "X", "quantity": 0, "unit_price": 10}]
	}`)
	if resp.StatusCode() < 400 {
		t.Fatalf("expected failure, got %d: %s", resp.StatusCode(), resp.Body())
	}

	store := sqladapter.StoreOf[Sale](db, "sales")
	rows, err := store.Find(context.Background(), store.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected rollback, got %d sales", len(rows))
	}
}

func TestAddItem_ToExistingSaleRecalculatesTotal(t *testing.T) {
	app, _ := testApp(t)
	tc := nitro.NewTestClient(app)

	id := checkout(t, tc, `{"customer_name": "Cid", "items": [{"sku": "TEA", "quantity": 1, "unit_price": 5}]}`)

	added := tc.Post(t, "/sales/"+id+"/sale_items", `{"sku": "CAKE", "quantity": 2, "unit_price": 12}`)
	tc.AssertStatus(t, added, 201)
	tc.AssertBodyContains(t, added, `"sku":"CAKE"`)

	got := tc.Get(t, "/sales/"+id)
	tc.AssertStatus(t, got, 200)
	tc.AssertBodyContains(t, got, `"total":29`) // 5 + 2*12
}

func TestUpdateItem_RecalculatesSaleTotal(t *testing.T) {
	app, _ := testApp(t)
	tc := nitro.NewTestClient(app)

	id := checkout(t, tc, `{"customer_name": "Deb", "items": [{"sku": "WATER", "quantity": 1, "unit_price": 3}]}`)

	list := tc.Get(t, "/sales/"+id+"/sale_items")
	tc.AssertStatus(t, list, 200)
	itemID := extractFirstItemID(t, list.Body())

	patched := tc.Patch(t, "/sales/"+id+"/sale_items/"+itemID, `{"quantity": 4}`)
	tc.AssertStatus(t, patched, 200)

	got := tc.Get(t, "/sales/"+id)
	tc.AssertStatus(t, got, 200)
	tc.AssertBodyContains(t, got, `"total":12`) // 4*3
}

func TestVoid_ThenRejectsFurtherCommands(t *testing.T) {
	app, _ := testApp(t)
	tc := nitro.NewTestClient(app)

	id := checkout(t, tc, `{"customer_name": "Eve", "items": [{"sku": "JUICE", "quantity": 1, "unit_price": 7}]}`)

	void := tc.Post(t, "/sales/"+id+"/void", "")
	tc.AssertStatus(t, void, 200)
	tc.AssertBodyContains(t, void, `"status":"voided"`)

	again := tc.Post(t, "/sales/"+id+"/void", "")
	if again.StatusCode() == 200 {
		t.Fatal("expected conflict voiding an already-voided sale")
	}
}

func TestTodayQuery_ListsTodaysSales(t *testing.T) {
	app, _ := testApp(t)
	tc := nitro.NewTestClient(app)

	id := checkout(t, tc, `{"customer_name": "Fay", "items": [{"sku": "SODA", "quantity": 1, "unit_price": 4}]}`)

	resp := tc.Get(t, "/sales/queries/today")
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, id)
}
