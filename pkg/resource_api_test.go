package nitro_test

import (
	"strings"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type resItem struct {
	ID    string `json:"id"`
	Title string `json:"title" validate:"required"`
	Done  bool   `json:"done"`
}

func TestAppResource_Config(t *testing.T) {
	repo := nitro.NewMemoryRepo[resItem]()
	app := nitro.New()

	app.Resource("/items", &resItem{}, nitro.AdaptRepo(repo), &nitro.ResourceConfig{
		NoCache:    true,
		Filterable: []string{"title", "done"},
		Sortable:   []string{"title"},
	})

	tc := nitro.NewTestClient(app)
	tc.AssertStatus(t, tc.Post(t, "/items", `{"title":"a","done":false}`), 201)
	list := tc.Get(t, "/items?title=a")
	tc.AssertStatus(t, list, 200)
	tc.AssertBodyContains(t, list, `"title":"a"`)
}

func TestResource_WriteMiddleware(t *testing.T) {
	repo := nitro.NewMemoryRepo[resItem]()
	app := nitro.New(nitro.Config{DisableDefaults: true})

	var writes, reads int
	markWrite := func(c *nitro.Ctx) error { writes++; return nil }
	markRead := func(c *nitro.Ctx) error { reads++; return nil }

	app.Resource("/items", &resItem{}, nitro.AdaptRepo(repo), &nitro.ResourceConfig{
		NoCache:         true,
		ReadMiddleware:  []nitro.Handler{markRead},
		WriteMiddleware: []nitro.Handler{markWrite},
	})

	tc := nitro.NewTestClient(app)
	tc.AssertStatus(t, tc.Get(t, "/items"), 200)
	tc.AssertStatus(t, tc.Post(t, "/items", `{"title":"x"}`), 201)
	if reads != 1 {
		t.Fatalf("reads=%d", reads)
	}
	if writes != 1 {
		t.Fatalf("writes=%d", writes)
	}
}

func TestGroup_Resource_InheritsMiddleware(t *testing.T) {
	repo := nitro.NewMemoryRepo[resItem]()
	app := nitro.New(nitro.Config{DisableDefaults: true})

	var hits int
	gate := func(c *nitro.Ctx) error { hits++; return nil }

	g := app.Group("/api", gate)
	g.Resource("/items", &resItem{}, nitro.AdaptRepo(repo), &nitro.ResourceConfig{NoCache: true})

	tc := nitro.NewTestClient(app)
	tc.AssertStatus(t, tc.Get(t, "/api/items"), 200)
	tc.AssertStatus(t, tc.Post(t, "/api/items", `{"title":"y"}`), 201)
	if hits < 2 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestAppResource_DefaultsWithoutOpts(t *testing.T) {
	repo := nitro.NewMemoryRepo[resItem]()
	app := nitro.New()
	app.Resource("/items", &resItem{}, nitro.AdaptRepo(repo))

	tc := nitro.NewTestClient(app)
	tc.AssertStatus(t, tc.Post(t, "/items", `{"title":"b"}`), 201)
}

func TestResourceConfig_ToOpts(t *testing.T) {
	cfg := &nitro.ResourceConfig{
		NoCache:    true,
		AutoID:     nitro.Ptr(false),
		Filterable: []string{"name"},
		FilterRules: []nitro.FilterRule{
			{Param: "q", Field: "name", Op: "icontains"},
		},
		Sortable: []string{"id"},
	}
	// exercise via Resource registration path
	repo := nitro.NewMemoryRepo[resItem]()
	app := nitro.New()
	app.Resource("/x", &resItem{}, nitro.AdaptRepo(repo), cfg)
	if cfg.AutoID == nil || *cfg.AutoID || !cfg.NoCache {
		t.Fatal("config mutated unexpectedly")
	}
}

func TestResource_OnlyAndExcept(t *testing.T) {
	repo := nitro.NewMemoryRepo[resItem]()
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource("/ro", &resItem{}, nitro.AdaptRepo(repo), &nitro.ResourceConfig{
		NoCache: true,
		Only:    []string{nitro.ActionList, nitro.ActionGet},
	})
	tc := nitro.NewTestClient(app)
	tc.AssertStatus(t, tc.Get(t, "/ro"), 200)
	tc.AssertStatus(t, tc.Post(t, "/ro", `{"title":"no"}`), 404)

	repo2 := nitro.NewMemoryRepo[resItem]()
	app2 := nitro.New(nitro.Config{DisableDefaults: true})
	app2.Resource("/full", &resItem{}, nitro.AdaptRepo(repo2), &nitro.ResourceConfig{
		NoCache: true,
		Except:  []string{nitro.ActionDelete},
	})
	tc2 := nitro.NewTestClient(app2)
	created := tc2.Post(t, "/full", `{"title":"y"}`)
	tc2.AssertStatus(t, created, 201)
	// delete should 404
	tc2.AssertStatus(t, tc2.Delete(t, "/full/missing-id"), 404)
}

func TestResource_Replace(t *testing.T) {
	repo := nitro.NewMemoryRepo[resItem]()
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.Resource("/items", &resItem{}, nitro.AdaptRepo(repo), &nitro.ResourceConfig{
		NoCache: true,
		Replace: map[string]nitro.Handler{
			nitro.ActionList: func(c *nitro.Ctx) error {
				return c.OK(nitro.Map{"custom": true})
			},
		},
	})
	tc := nitro.NewTestClient(app)
	res := tc.Get(t, "/items")
	tc.AssertStatus(t, res, 200)
	tc.AssertBodyContains(t, res, `"custom":true`)
}

func TestResource_AcceptsMemoryRepoDirectly(t *testing.T) {
	type Gadget struct {
		nitro.Model
		Name string `json:"name" validate:"required"`
	}
	app := nitro.New(nitro.Config{DisableDefaults: true, Env: nitro.EnvDevelopment})
	app.Resource(&Gadget{}, nitro.NewMemoryRepo[Gadget](), &nitro.ResourceConfig{Public: true, NoCache: true})
	if err := app.Err(); err != nil {
		t.Fatal(err)
	}
	r, err := app.TestWithHeaders("POST", "/gadgets", strings.NewReader(`{"name":"x"}`), map[string]string{"Content-Type": "application/json"})
	if err != nil || r.StatusCode() != 201 {
		t.Fatalf("%v %d", err, r.StatusCode())
	}
}
