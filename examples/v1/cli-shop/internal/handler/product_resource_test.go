package handler_test

import (
	"strings"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cli-shop/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cli-shop/v2/internal/model"
)

func TestProductResourceCRUD(t *testing.T) {
	app := nitro.New(nitro.Config{DisableDefaults: true, Env: nitro.EnvDevelopment})
	handler.RegisterProduct(app, nitro.NewMemoryRepo[model.Product]())
	if err := app.Err(); err != nil {
		t.Fatal(err)
	}
	tc := nitro.NewTestClient(app)
	c := tc

	created := c.Post(t, "/products", "{\"name\":\"sample\",\"price\":1}")
	tc.AssertStatus(t, created, 201)

	list := c.Get(t, "/products")
	tc.AssertStatus(t, list, 200)
	if !strings.Contains(string(list.Body()), "sample") && !strings.Contains(string(list.Body()), "\"id\"") {
		t.Fatalf("list must contain the created row: %s", list.Body())
	}
}
