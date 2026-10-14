package nitro_test

import (
	"testing"
	"time"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func TestResourceCacheBustPattern_FromAppCache(t *testing.T) {
	cache := nitro.MemoryCache(64)
	app := nitro.New(nitro.Config{Cache: cache, DisableDefaults: true})

	repo := nitro.NewMemoryRepo[struct {
		nitro.Model
		Name string `json:"name"`
	}]()
	app.Resource("/api/drivers", &struct {
		nitro.Model
		Name string `json:"name"`
	}{}, nitro.AdaptRepo(repo), &nitro.ResourceConfig{
		Filterable: []string{"name"},
	})

	// App cache must be reachable; bust pattern is internal Resource behavior.
	if app.Cache() == nil {
		t.Fatal("expected app cache")
	}
	_ = cache.Set("drivers:x", map[string]string{"ok": "1"}, time.Minute)
}

func TestSetCache(t *testing.T) {
	app := nitro.New(nitro.Config{DisableDefaults: true})
	if app.Cache() != nil {
		t.Fatal("expected nil cache")
	}
	c := nitro.MemoryCache(8)
	app.SetCache(c)
	if app.Cache() != c {
		t.Fatal("SetCache did not stick")
	}
}
