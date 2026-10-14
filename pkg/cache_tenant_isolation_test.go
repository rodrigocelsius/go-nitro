package nitro

import (
	"strings"
	"testing"
)

type cacheOrder struct {
	TenantModel
	Title string `json:"title"`
}

func TestResourceCache_TenantIsolationAndInvalidation(t *testing.T) {
	app := New(Config{DisableDefaults: true, Env: EnvDevelopment})
	allow := true
	app.Use(TenantMiddleware(TenantConfig{Require: true, AllowHeader: &allow}))
	app.Resource(&cacheOrder{}, NewMemoryRepo[cacheOrder](), &ResourceConfig{Public: true}) // cache ON (default 5m)
	post := func(tn, body string) {
		r, _ := app.TestWithHeaders("POST", "/cache_orders", strings.NewReader(body), map[string]string{"Content-Type": "application/json", "X-Tenant-ID": tn})
		if r.StatusCode() != 201 {
			t.Fatalf("post %d %s", r.StatusCode(), r.Body())
		}
	}
	list := func(tn string) string {
		r, _ := app.TestWithHeaders("GET", "/cache_orders", nil, map[string]string{"X-Tenant-ID": tn})
		return string(r.Body())
	}
	A, B := "01900000-0000-7000-8000-00000000000a", "01900000-0000-7000-8000-00000000000b"
	post(A, `{"title":"a1"}`)
	_ = list(A) // warm cache
	// Match the title field, not a bare substring: random UUIDs in the body
	// may contain "a1"/"a2" by chance (this test used to flake on that).
	has := func(body, title string) bool { return strings.Contains(body, `"title":"`+title+`"`) }
	if has(list(B), "a1") {
		t.Fatal("tenant B sees tenant A via cache")
	}
	post(A, `{"title":"a2"}`)
	if !has(list(A), "a2") {
		t.Fatal("cache not invalidated after create")
	}
	post(B, `{"title":"b1"}`)
	if l := list(B); !has(l, "b1") || has(l, "a2") {
		t.Fatalf("B list wrong: %s", l)
	}
}
