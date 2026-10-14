package main

import (
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func buildApp() *nitro.App {
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Post("/contacts", func(c *nitro.Ctx) error {
		in, err := nitro.Parse[Contact](c)
		if err != nil {
			return err
		}
		return c.Created(in)
	})
	return app
}

func TestTransformOnParse(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Post(t, "/contacts", `{"name":"  joão da silva ","phone":"(11) 98877-6655","uf":"sp"}`)
	tc.AssertStatus(t, resp, 201)
	tc.AssertBodyContains(t, resp, `"name":"João da Silva"`)
	tc.AssertBodyContains(t, resp, `"phone":"11988776655"`)
	tc.AssertBodyContains(t, resp, `"uf":"SP"`)
}
