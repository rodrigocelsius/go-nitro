package nitro_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type l2Driver struct {
	nitro.Model
	Name   string `json:"name"`
	Secret string `json:"-"`
}

// countingRepo counts reads that reach the backing repository.
type countingRepo struct {
	nitro.TypedRepository[l2Driver]
	finds, lists atomic.Int32
}

func (r *countingRepo) FindByID(ctx context.Context, id any) (*l2Driver, error) {
	r.finds.Add(1)
	return r.TypedRepository.FindByID(ctx, id)
}

func (r *countingRepo) List(ctx context.Context, page, perPage int) (*nitro.ResourceListResult, error) {
	r.lists.Add(1)
	return r.TypedRepository.List(ctx, page, perPage)
}

// brokenCache simulates Redis being down: every call fails.
type brokenCache struct{ calls atomic.Int32 }

func (b *brokenCache) Get(string, any) error {
	b.calls.Add(1)
	return errors.New("dial tcp: connection refused")
}
func (b *brokenCache) Set(string, any, time.Duration) error {
	b.calls.Add(1)
	return errors.New("dial tcp: connection refused")
}
func (b *brokenCache) Delete(string) error     { return errors.New("down") }
func (b *brokenCache) Invalidate(string) error { return errors.New("down") }
func (b *brokenCache) Clear() error            { return errors.New("down") }

func newL2App(t *testing.T, c nitro.Cache) (*nitro.App, *countingRepo, *l2Driver) {
	t.Helper()
	mem := nitro.NewMemoryRepo[l2Driver]()
	d := &l2Driver{Name: "Ada", Secret: "s3cret"}
	if err := mem.Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	repo := &countingRepo{TypedRepository: mem}
	app := nitro.New(nitro.Config{DisableDefaults: true})
	app.SetCache(c)
	app.Resource("/drivers", &l2Driver{}, nitro.AdaptRepo[l2Driver](repo), &nitro.ResourceConfig{Public: true})
	return app, repo, d
}

func TestResourceL2_ReadThroughSharedCache(t *testing.T) {
	shared := nitro.MemoryCache(256)
	app, repo, d := newL2App(t, shared)
	tc := nitro.NewTestClient(app)

	for i := 0; i < 3; i++ {
		resp := tc.Get(t, "/drivers/"+d.ID.String())
		tc.AssertStatus(t, resp, 200)
		tc.AssertBodyContains(t, resp, `"name":"Ada"`)
	}
	if got := repo.finds.Load(); got != 1 {
		t.Fatalf("repository FindByID calls = %d, want 1 (served from L2)", got)
	}

	// A second replica sharing the cache never touches its repository.
	app2, repo2, _ := newL2App(t, shared)
	tc2 := nitro.NewTestClient(app2)
	tc2.AssertStatus(t, tc2.Get(t, "/drivers/"+d.ID.String()), 200)
	if repo2.finds.Load() != 0 {
		t.Fatal("replica 2 should read replica 1's cached entry")
	}

	tc.AssertStatus(t, tc.Get(t, "/drivers"), 200)
	tc.AssertStatus(t, tc.Get(t, "/drivers"), 200)
	if got := repo.lists.Load(); got > 1 {
		t.Fatalf("repository List calls = %d, want ≤ 1", got)
	}

	// A write busts the shared entries: next read reaches the repository.
	tc.AssertStatus(t, tc.Patch(t, "/drivers/"+d.ID.String(), `{"name":"Grace"}`), 200)
	resp := tc.Get(t, "/drivers/"+d.ID.String())
	tc.AssertBodyContains(t, resp, `"name":"Grace"`)
}

// json:"-" fields are not in the shared cache; the PATCH path must load the
// row from the repository so the merge does not write them back empty.
func TestResourceL2_HiddenFieldsSurvivePatch(t *testing.T) {
	app, repo, d := newL2App(t, nitro.MemoryCache(256))
	tc := nitro.NewTestClient(app)
	tc.AssertStatus(t, tc.Get(t, "/drivers/"+d.ID.String()), 200) // fill L2
	tc.AssertStatus(t, tc.Patch(t, "/drivers/"+d.ID.String(), `{"name":"Grace"}`), 200)
	got, err := repo.TypedRepository.FindByID(context.Background(), d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Secret != "s3cret" {
		t.Fatalf("json:\"-\" field lost through the cache: %q", got.Secret)
	}
}

func TestResourceL2_CacheDownFallsBackToRepository(t *testing.T) {
	broken := &brokenCache{}
	app, repo, d := newL2App(t, broken)
	tc := nitro.NewTestClient(app)
	for i := 0; i < 2; i++ {
		resp := tc.Get(t, "/drivers/"+d.ID.String())
		tc.AssertStatus(t, resp, 200)
		if !strings.Contains(string(resp.Body()), "Ada") {
			t.Fatalf("body: %s", resp.Body())
		}
	}
	if repo.finds.Load() != 2 || broken.calls.Load() != 1 {
		t.Fatalf("finds=%d cache calls=%d: after one failure the cache must be skipped (circuit open)", repo.finds.Load(), broken.calls.Load())
	}
	tc.AssertStatus(t, tc.Patch(t, "/drivers/"+d.ID.String(), `{"name":"Grace"}`), 200)
}

func TestL2CacheRepo_DirectUse(t *testing.T) {
	mem := nitro.NewMemoryRepo[l2Driver]()
	ctx := context.Background()
	d := &l2Driver{Name: "Ada"}
	_ = mem.Create(ctx, d)
	repo := nitro.L2CacheRepo(nitro.AdaptRepo[l2Driver](mem), nitro.MemoryCache(16), "drivers:", time.Minute)
	if _, err := repo.FindByID(ctx, d.ID); err != nil { // learns model + fills
		t.Fatal(err)
	}
	got, err := repo.FindByID(ctx, d.ID)
	if err != nil || got.(*l2Driver).Name != "Ada" {
		t.Fatalf("got %#v err=%v", got, err)
	}
}
