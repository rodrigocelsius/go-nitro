package main

import (
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func TestResourceDX_BindMapAndHealth(t *testing.T) {
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.HealthCheck(nitro.NamedHealth("ok", func() error { return nil }))
	app.Post("/echo-map", func(c *nitro.Ctx) error {
		m, err := c.BindMap()
		if err != nil {
			return err
		}
		return c.OK(m)
	})
	repo := nitro.NewMemoryRepo[Item]()
	nitro.Resource(app, "/items", repo, &nitro.ResourceConfig{
		NoCache:     true,
		FilterRules: []nitro.FilterRule{nitro.FilterActive()},
		ExtraRoutes: []nitro.ResourceExtraRoute{
			{Method: "GET", Path: "/export", Handler: func(c *nitro.Ctx) error {
				return c.Text("csv")
			}},
		},
	})

	tc := nitro.NewTestClient(app)
	r := tc.Get(t, "/health")
	tc.AssertStatus(t, r, 200)

	r = tc.Post(t, "/echo-map", `{"x":1}`)
	tc.AssertStatus(t, r, 200)
	tc.AssertBodyContains(t, r, "x")

	r = tc.Post(t, "/items", `{"name":"ada"}`)
	tc.AssertStatus(t, r, 201)

	r = tc.Get(t, "/items/export")
	tc.AssertStatus(t, r, 200)
}
