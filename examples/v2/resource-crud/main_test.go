package main

import (
	"context"
	"encoding/json"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func setup(t *testing.T) *nitro.TestClient {
	t.Helper()
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.MigrateFS(ctx, migrations); err != nil {
		t.Fatal(err)
	}
	return nitro.NewTestClient(newApp(db))
}

func id(t *testing.T, body []byte) string {
	var m struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &m); err != nil || m.ID == "" {
		t.Fatalf("no id in %s", body)
	}
	return m.ID
}

func TestProducts_CRUDFiltersAndStats(t *testing.T) {
	tc := setup(t)
	cat := tc.Post(t, "/categories", `{"name":"  office chairs "}`)
	tc.AssertStatus(t, cat, 201)
	tc.AssertBodyContains(t, cat, `"name":"Office Chairs"`) // transform
	catID := id(t, cat.Body())

	p := tc.Post(t, "/products", `{"name":"Chair","sku":" ch-1 ","price":120,"stock":3,"active":true,"category_id":"`+catID+`","category_name":"hacked"}`)
	tc.AssertStatus(t, p, 201)
	tc.AssertBodyContains(t, p, `"sku":"CH-1"`)
	tc.AssertBodyContains(t, p, `"category_name":"Office Chairs"`) // computed, client value ignored
	pid := id(t, p.Body())
	tc.AssertStatus(t, tc.Post(t, "/products", `{"name":"Desk","sku":"DK-1","price":300,"stock":0,"active":true}`), 201)
	tc.AssertStatus(t, tc.Post(t, "/products", `{"name":"Lamp","sku":"LP-1","price":15,"stock":7}`), 201)
	tc.AssertStatus(t, tc.Post(t, "/products", `{"name":"x","sku":"","price":0}`), 422)

	list := tc.Get(t, "/products?q=chair&min_price=100")
	tc.AssertStatus(t, list, 200)
	tc.AssertBodyContains(t, list, `"total":1`)
	sorted := tc.Get(t, "/products?sort=-price&per_page=1")
	tc.AssertBodyContains(t, sorted, `"name":"Desk"`)

	tc.AssertBodyContains(t, tc.Patch(t, "/products/"+pid, `{"price":99}`), `"name":"Chair"`) // merge keeps fields
	stats := tc.Get(t, "/products/queries/stats")
	tc.AssertBodyContains(t, stats, `"total":3`)
	tc.AssertBodyContains(t, stats, `"active":2`)
	tc.AssertBodyContains(t, stats, `"out_of_stock":1`)
	tc.AssertBodyContains(t, stats, `"units":10`)

	tc.AssertStatus(t, tc.Delete(t, "/products/"+pid), 204)
	tc.AssertStatus(t, tc.Get(t, "/products/"+pid), 404) // soft-deleted
	tc.AssertBodyContains(t, tc.Get(t, "/products"), `"total":2`)
}
