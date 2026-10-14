package handler_test

import (
	"strings"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cli-shop/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v1/cli-shop/v2/internal/model"
)

func TestInvoiceResourceCRUD(t *testing.T) {
	app := nitro.New(nitro.Config{DisableDefaults: true, Env: nitro.EnvDevelopment})
	allowHeader := true
	app.Use(nitro.TenantMiddleware(nitro.TenantConfig{Require: true, AllowHeader: &allowHeader}))
	handler.Register(app, nitro.NewMemoryRepo[model.Invoice]())
	if err := app.Err(); err != nil {
		t.Fatal(err)
	}
	tc := nitro.NewTestClient(app)
	c := tc.WithHeader("X-Tenant-ID", "01900000-0000-7000-8000-000000000001")

	created := c.Post(t, "/invoices", "{\"total\":1,\"customer\":\"sample\"}")
	tc.AssertStatus(t, created, 201)

	list := c.Get(t, "/invoices")
	tc.AssertStatus(t, list, 200)
	if !strings.Contains(string(list.Body()), "sample") && !strings.Contains(string(list.Body()), "\"id\"") {
		t.Fatalf("list must contain the created row: %s", list.Body())
	}
}
