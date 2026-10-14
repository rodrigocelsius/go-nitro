package handler_test

import (
	"context"
	"encoding/json"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch-blinq/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch-blinq/v2/internal/model"
)

const productSchemaSQLite = `
CREATE TABLE IF NOT EXISTS products (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    price REAL NOT NULL DEFAULT 0,
    active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);
`

func openProductStore(t *testing.T) (*sqladapter.DB, *sqladapter.Store[model.Product]) {
	t.Helper()
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatalf("OpenTest: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Gateway().Exec(ctx, "schema", productSchemaSQLite, nil); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db, sqladapter.StoreOf[model.Product](db, "products")
}

func buildApp(t *testing.T) *nitro.App {
	t.Helper()
	_, store := openProductStore(t)
	app := nitro.New()
	handler.Register(app, store)
	return app
}

func TestProductStore_CreateOne(t *testing.T) {
	ctx := context.Background()
	_, store := openProductStore(t)
	got, err := store.CreateOne(ctx, &model.Product{Name: "Widget", Price: 9.99, Active: true})
	if err != nil {
		t.Fatalf("CreateOne: %v", err)
	}
	if got.ID.IsZero() {
		t.Fatal("expected generated id")
	}
}

func TestCreateProduct(t *testing.T) {
	tc := nitro.NewTestClient(buildApp(t))
	resp := tc.Post(t, "/products", `{"name":"Widget","price":9.99}`)
	tc.AssertStatus(t, resp, 201)
	tc.AssertBodyContains(t, resp, "Widget")
}

func TestCreateProduct_EmptyName(t *testing.T) {
	tc := nitro.NewTestClient(buildApp(t))
	resp := tc.Post(t, "/products", `{"name":"","price":9.99}`)
	tc.AssertStatus(t, resp, 422)
}

func TestListProducts(t *testing.T) {
	tc := nitro.NewTestClient(buildApp(t))
	tc.Post(t, "/products", `{"name":"Keyboard","price":1,"active":true}`)
	tc.Post(t, "/products", `{"name":"Mouse","price":2,"active":false}`)
	resp := tc.Get(t, "/products")
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, "Keyboard")
	tc.AssertBodyContains(t, resp, "Mouse")

	resp = tc.Get(t, "/products?active=true&q=key")
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, "Keyboard")
}

func TestDeleteProduct(t *testing.T) {
	tc := nitro.NewTestClient(buildApp(t))
	created := tc.Post(t, "/products", `{"name":"Xyzzy","price":1}`)
	id := extractID(t, created.Body())
	resp := tc.Delete(t, "/products/"+id)
	tc.AssertStatus(t, resp, 204)
	resp = tc.Get(t, "/products/"+id)
	tc.AssertStatus(t, resp, 404)
}

func extractID(t *testing.T, body []byte) string {
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
