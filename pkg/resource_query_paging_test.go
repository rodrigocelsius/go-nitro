package nitro_test

import (
	"context"
	"encoding/json"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// Resource Queries used to pass ?page / ?per_page straight through: a client
// could ask for an unbounded page or a negative offset.
func TestResourceQuery_PageIsClamped(t *testing.T) {
	var got nitro.Page
	repo := nitro.NewMemoryRepo[bizOrder]()
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource("/orders", &bizOrder{}, nitro.AdaptRepo(repo), &nitro.ResourceConfig{
		NoCache:    true,
		MaxPerPage: 50,
		Queries: nitro.Queries(nitro.Q("probe", func(_ context.Context, p nitro.Page) (nitro.Page, error) {
			got = p
			return p, nil
		})),
	})
	tc := nitro.NewTestClient(app)

	cases := []struct {
		query         string
		page, perPage int
	}{
		{"?per_page=100000000&page=-5", 1, 50},
		{"?per_page=0&page=99999999", nitro.DefaultListMaxPage, nitro.DefaultListPerPage},
		{"?per_page=10&page=3", 3, 10},
	}
	for _, c := range cases {
		resp := tc.Get(t, "/orders/queries/probe"+c.query)
		tc.AssertStatus(t, resp, 200)
		if got.Number != c.page || got.PerPage != c.perPage {
			body, _ := json.Marshal(got)
			t.Errorf("%s: page=%s, want number=%d per_page=%d", c.query, body, c.page, c.perPage)
		}
	}
}
