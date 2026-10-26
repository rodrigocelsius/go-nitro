package handler_test

import (
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v2/cli-shop/v2/internal/repository"
)

func TestListProductEmpty(t *testing.T) {
	app := nitro.New()
	handler.Register(app, repository.NewProductMemory())
	tc := nitro.NewTestClient(app)
	resp := tc.Get(t, "/products")
	tc.AssertStatus(t, resp, 200)
}
