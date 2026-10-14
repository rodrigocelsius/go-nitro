package handler_test

import (
	"encoding/json"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2/internal/repository"
)

func buildApp() *nitro.App {
	repo := repository.NewProductMemory()
	app := nitro.New()
	handler.Register(app, repo)
	return app
}

func TestCreateProduct(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Post(t, "/products", `{"name":"Widget","price":9.99}`)
	tc.AssertStatus(t, resp, 201)
	tc.AssertBodyContains(t, resp, "Widget")
}

func TestCreateProduct_EmptyName(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Post(t, "/products", `{"name":"","price":9.99}`)
	tc.AssertStatus(t, resp, 422)
}

func TestListProducts(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	tc.Post(t, "/products", `{"name":"A","price":1}`)
	tc.Post(t, "/products", `{"name":"B","price":2}`)
	resp := tc.Get(t, "/products")
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, "A")
	tc.AssertBodyContains(t, resp, "B")
}

func TestApplyDiscount(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	created := tc.Post(t, "/products", `{"name":"Sale","price":100}`)
	tc.AssertStatus(t, created, 201)
	id := extractID(t, created.Body())
	resp := tc.Post(t, "/products/"+id+"/discount", `{"percent":10}`)
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, "90")
}

func TestDeleteProduct(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	created := tc.Post(t, "/products", `{"name":"X","price":1}`)
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
