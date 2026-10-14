package main

import (
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func buildApp() *nitro.App {
	app := nitro.New()

	api := app.Group("/api")
	api.Get("/ping", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"message": "pong"})
	})
	api.Get("/users/:id", getUser)
	api.Post("/users", createUser)

	app.Get("/hello/:name", func(c *nitro.Ctx) error {
		return c.Text("Hello " + c.Param("name"))
	})
	app.Get("/", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"service": "nitro-v1-basic"})
	})
	return app
}

func TestRoot(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Get(t, "/")
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, "nitro-v1-basic")
}

func TestPing(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Get(t, "/api/ping")
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, "pong")
}

func TestHello(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Get(t, "/hello/nitro")
	tc.AssertStatus(t, resp, 200)
	if string(resp.Body()) != "Hello nitro" {
		t.Fatalf("body = %q, want 'Hello nitro'", resp.Body())
	}
}

func TestGetUser(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Get(t, "/api/users/42")
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, "42")
}

func TestGetUser_NotFound(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Get(t, "/api/users/999")
	tc.AssertStatus(t, resp, 404)
}

func TestCreateUser(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Post(t, "/api/users", `{"name":"Jane"}`)
	tc.AssertStatus(t, resp, 201)
	tc.AssertBodyContains(t, resp, "Jane")
}

func TestCreateUser_EmptyName(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Post(t, "/api/users", `{"name":""}`)
	tc.AssertStatus(t, resp, 422)
}

func TestCreateUser_InvalidJSON(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Post(t, "/api/users", `not json`)
	tc.AssertStatus(t, resp, 400)
}
