package handler_test

import (
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch-blinq/v2/internal/handler"
)

func TestHealth(t *testing.T) {
	app := nitro.New()
	app.Get("/health", handler.Health)
	tc := nitro.NewTestClient(app)
	resp := tc.Get(t, "/health")
	tc.AssertStatus(t, resp, 200)
}
