package main

import (
	"encoding/json"
	"testing"
	"time"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func buildApp() *nitro.App {
	app := nitro.New()
	nitro.Resource(app, "/todos", nitro.NewMemoryRepo[Todo]())
	return app
}

func TestCRUD_Create(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Post(t, "/todos", `{"title":"Buy milk"}`)
	tc.AssertStatus(t, resp, 201)
	tc.AssertBodyContains(t, resp, "Buy milk")

	var m map[string]any
	if err := json.Unmarshal(resp.Body(), &m); err != nil {
		t.Fatal(err)
	}
	if id, _ := m["id"].(string); id == "" {
		t.Fatal("expected auto-generated id")
	}
	if _, ok := m["created_at"]; !ok {
		t.Fatal("expected created_at")
	}
}

func TestCRUD_List(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	tc.Post(t, "/todos", `{"title":"AA"}`)
	tc.Post(t, "/todos", `{"title":"BB"}`)

	resp := tc.Get(t, "/todos")
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, "AA")
	tc.AssertBodyContains(t, resp, "BB")
}

func TestCRUD_GetByID(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	created := tc.Post(t, "/todos", `{"title":"Find me"}`)
	tc.AssertStatus(t, created, 201)
	id := extractID(t, created.Body())

	resp := tc.Get(t, "/todos/"+id)
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, "Find me")
}

func TestCRUD_Update(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	created := tc.Post(t, "/todos", `{"title":"Old"}`)
	id := extractID(t, created.Body())

	resp := tc.Patch(t, "/todos/"+id, `{"title":"New","done":true}`)
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, "New")
}

func TestCRUD_Delete(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	created := tc.Post(t, "/todos", `{"title":"Delete me"}`)
	id := extractID(t, created.Body())

	resp := tc.Delete(t, "/todos/"+id)
	if resp.StatusCode() != 200 && resp.StatusCode() != 204 {
		t.Fatalf("status = %d, want 200 or 204", resp.StatusCode())
	}

	resp = tc.Get(t, "/todos/"+id)
	tc.AssertStatus(t, resp, 404)
}

func TestCRUD_SoftDeleteHiddenFromList(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	created := tc.Post(t, "/todos", `{"title":"Gone"}`)
	id := extractID(t, created.Body())
	tc.Delete(t, "/todos/"+id)

	resp := tc.Get(t, "/todos")
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, `"total":0`)
}

func TestCRUD_NotFound(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Get(t, "/todos/999")
	tc.AssertStatus(t, resp, 404)
}

func TestCRUD_CreateValidation(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())

	resp := tc.Post(t, "/todos", `{}`)
	tc.AssertStatus(t, resp, 422)
	tc.AssertBodyContains(t, resp, "title")

	resp = tc.Post(t, "/todos", `{"title":"x"}`)
	tc.AssertStatus(t, resp, 422)

	resp = tc.Post(t, "/todos", `{`)
	tc.AssertStatus(t, resp, 400)
}

func TestCRUD_UpdateNotFound(t *testing.T) {
	tc := nitro.NewTestClient(buildApp())
	resp := tc.Patch(t, "/todos/999", `{"title":"Nope"}`)
	tc.AssertStatus(t, resp, 404)
}

func TestCRUD_MemoryRepoCached(t *testing.T) {
	app := nitro.New()
	// Customize: explicit CacheRepo (own TTL; Resource does not wrap again).
	app.Resource("/todos", &Todo{}, nitro.CacheRepo(nitro.MemoryRepo[Todo](), time.Minute))
	tc := nitro.NewTestClient(app)
	tc.AssertStatus(t, tc.Post(t, "/todos", `{"title":"Cached"}`), 201)
}

func TestCRUD_WithCacheOpt(t *testing.T) {
	app := nitro.New()
	nitro.Resource(app, "/todos", nitro.NewMemoryRepo[Todo](), &nitro.ResourceConfig{CacheTTL: time.Minute})
	tc := nitro.NewTestClient(app)
	tc.AssertStatus(t, tc.Post(t, "/todos", `{"title":"Opts"}`), 201)
}

func extractID(t *testing.T, body []byte) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	id, _ := m["id"].(string)
	if id == "" {
		t.Fatalf("missing id in body: %s", body)
	}
	return id
}
