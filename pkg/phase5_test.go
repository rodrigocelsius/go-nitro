package nitro_test

import (
	"context"
	"strings"
	"testing"
	"time"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type phase5Item struct {
	ID   string `json:"id"`
	Name string `json:"name" validate:"required"`
}

type phase5Repo struct {
	items *nitro.SafeMap[string, *phase5Item]
}

func newPhase5Repo() *phase5Repo {
	return &phase5Repo{items: nitro.NewSafeMap[string, *phase5Item]()}
}

func (r *phase5Repo) FindByID(_ context.Context, id any) (*phase5Item, error) {
	v, ok := r.items.Get(id.(string))
	if !ok {
		return nil, nitro.NotFound("not found")
	}
	return v, nil
}

func (r *phase5Repo) List(_ context.Context, page, perPage int) (*nitro.ResourceListResult, error) {
	all := r.items.Values()
	p := nitro.Paginate(len(all), page, perPage)
	return &nitro.ResourceListResult{
		Items: nitro.ToAny(nitro.PageSlice(all, p)), Total: p.Total, Page: p.Page, PerPage: p.PerPage, TotalPages: p.TotalPages,
	}, nil
}

func (r *phase5Repo) Create(_ context.Context, e *phase5Item) error {
	e.ID = nitro.NextIDString()
	r.items.Set(e.ID, e)
	return nil
}

func (r *phase5Repo) Update(_ context.Context, e *phase5Item) error {
	r.items.Set(e.ID, e)
	return nil
}

func (r *phase5Repo) Delete(_ context.Context, id any) error {
	r.items.Delete(id.(string))
	return nil
}

func TestPhase5_TypedRepoAndCache(t *testing.T) {
	repo := newPhase5Repo()
	app := nitro.New()
	app.Resource("/items", &phase5Item{}, nitro.CacheRepo(nitro.AdaptRepo(repo), time.Minute))

	tc := nitro.NewTestClient(app)
	resp := tc.Post(t, "/items", `{"name":"alpha"}`)
	tc.AssertStatus(t, resp, 201)
	tc.AssertBodyContains(t, resp, "alpha")
}

func TestPhase5_ParseAndIDs(t *testing.T) {
	app := nitro.New()
	app.Post("/echo", func(c *nitro.Ctx) error {
		req, err := nitro.Parse[phase5Item](c)
		if err != nil {
			return err
		}
		req.ID = nitro.NewUUIDString()
		return c.Created(req)
	})

	tc := nitro.NewTestClient(app)
	resp := tc.Post(t, "/echo", `{"name":"x"}`)
	tc.AssertStatus(t, resp, 201)
	if !strings.Contains(string(resp.Body()), "-") {
		t.Fatalf("expected uuid in body: %s", resp.Body())
	}
}

func TestPhase5_FastCtxIsContext(t *testing.T) {
	app := nitro.New()
	app.Get("/ctx", func(c *nitro.Ctx) error {
		var ctx context.Context = c
		_ = ctx
		return c.OK(nitro.Map{"ok": true})
	})
	tc := nitro.NewTestClient(app)
	resp := tc.Get(t, "/ctx")
	tc.AssertStatus(t, resp, 200)
}
