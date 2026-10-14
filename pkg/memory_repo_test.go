package nitro

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type memTodo struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type memItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func TestMemoryRepo_CRUD(t *testing.T) {
	repo := NewMemoryRepo[memItem]()
	if repo.SoftDeleteEnabled() {
		t.Fatal("memItem should not soft-delete")
	}

	item := &memItem{ID: "1", Title: "a"}
	if err := repo.Create(context.Background(), item); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByID(context.Background(), "1")
	if err != nil || got.Title != "a" {
		t.Fatalf("got %+v err=%v", got, err)
	}

	item.Title = "b"
	if err := repo.Update(context.Background(), item); err != nil {
		t.Fatal(err)
	}

	list, err := repo.List(context.Background(), 1, 10)
	if err != nil || list.Total != 1 {
		t.Fatalf("list=%+v err=%v", list, err)
	}

	if err := repo.Delete(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
	if repo.Len() != 0 {
		t.Fatalf("expected hard delete, len=%d", repo.Len())
	}
	if _, err := repo.FindByID(context.Background(), "1"); err == nil {
		t.Fatal("expected not found")
	}
}

func TestMemoryRepo_SoftDelete(t *testing.T) {
	repo := NewMemoryRepo[memTodo]()
	if !repo.SoftDeleteEnabled() {
		t.Fatal("memTodo should soft-delete")
	}

	todo := &memTodo{ID: "1", Title: "x", CreatedAt: time.Now()}
	_ = repo.Create(context.Background(), todo)

	if err := repo.Delete(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
	if repo.Len() != 1 {
		t.Fatalf("soft delete should keep row, len=%d", repo.Len())
	}
	if _, err := repo.FindByID(context.Background(), "1"); err == nil {
		t.Fatal("soft-deleted should be not found")
	}
	list, _ := repo.List(context.Background(), 1, 10)
	if list.Total != 0 {
		t.Fatalf("list should hide soft-deleted, total=%d", list.Total)
	}
}

func TestMemoryRepo_AdaptAndResource(t *testing.T) {
	app := New()
	Resource(app, "/items", NewMemoryRepo[memItem]())

	tc := NewTestClient(app)
	resp := tc.Post(t, "/items", `{"title":"hi"}`)
	tc.AssertStatus(t, resp, 201)
	tc.AssertBodyContains(t, resp, "hi")

	id := extractJSONID(t, resp.Body())
	resp = tc.Get(t, "/items/"+id)
	tc.AssertStatus(t, resp, 200)

	resp = tc.Delete(t, "/items/"+id)
	if resp.StatusCode() != 200 && resp.StatusCode() != 204 {
		t.Fatalf("status=%d", resp.StatusCode())
	}
	resp = tc.Get(t, "/items/"+id)
	tc.AssertStatus(t, resp, 404)
}

func TestMemoryRepo_SoftDeleteViaHTTP(t *testing.T) {
	app := New()
	Resource(app, "/todos", NewMemoryRepo[memTodo]())

	tc := NewTestClient(app)
	created := tc.Post(t, "/todos", `{"title":"bye"}`)
	tc.AssertStatus(t, created, 201)
	id := extractJSONID(t, created.Body())

	del := tc.Delete(t, "/todos/"+id)
	if del.StatusCode() != 200 && del.StatusCode() != 204 {
		t.Fatalf("status=%d", del.StatusCode())
	}
	tc.AssertStatus(t, tc.Get(t, "/todos/"+id), 404)
	list := tc.Get(t, "/todos")
	tc.AssertStatus(t, list, 200)
	tc.AssertBodyContains(t, list, `"total":0`)
}

type ownedItem struct {
	ID     string `json:"id"`
	UserID string `json:"user_id" readonly:"true"`
	Title  string `json:"title"`
}

// TestResource_OwnerCheckPreventsIDOR verifies that ResourceConfig.OwnerCheck
// blocks Get/Update/Delete on an :id that exists but does not belong to the
// caller — the classic IDOR gap for app.Resource's default CRUD (vuln.md V-05).
func TestResource_OwnerCheckPreventsIDOR(t *testing.T) {
	app := New(Config{DisableDefaults: true})
	app.Resource("/orders", &ownedItem{}, MemoryRepo[ownedItem](), &ResourceConfig{
		BeforeCreate: func(_ context.Context, entity any) error {
			// Normally set from an authenticated claim; hardcoded here for the "owner" seed item.
			entity.(*ownedItem).UserID = "owner-1"
			return nil
		},
		OwnerCheck: func(c *domain.Ctx, entity any) bool {
			return entity.(*ownedItem).UserID == c.HeaderParam("X-User-ID")
		},
	})

	tc := NewTestClient(app)
	created := tc.Post(t, "/orders", `{"title":"mine"}`)
	tc.AssertStatus(t, created, 201)
	id := extractJSONID(t, created.Body())

	owner := tc.WithHeader("X-User-ID", "owner-1")
	attacker := tc.WithHeader("X-User-ID", "attacker-2")

	// The owner (matching header) can read it.
	tc.AssertStatus(t, owner.Get(t, "/orders/"+id), 200)

	// A different authenticated caller must not see, edit, or delete it —
	// even though the ID is valid and guessable/observed elsewhere.
	tc.AssertStatus(t, attacker.Get(t, "/orders/"+id), 404)
	tc.AssertStatus(t, attacker.Patch(t, "/orders/"+id, `{"title":"pwned"}`), 404)
	tc.AssertStatus(t, attacker.Delete(t, "/orders/"+id), 404)

	// Still there, untouched, for the real owner.
	getAfter := owner.Get(t, "/orders/"+id)
	tc.AssertStatus(t, getAfter, 200)
	tc.AssertBodyContains(t, getAfter, "mine")
}

func TestMemoryRepo_Constructor(t *testing.T) {
	adapted := MemoryRepo[memItem]()
	if adapted == nil {
		t.Fatal("nil")
	}
	app := New()
	app.Resource("/x", &memItem{}, CacheRepo(adapted, time.Minute))
	tc := NewTestClient(app)
	tc.AssertStatus(t, tc.Post(t, "/x", `{"title":"z"}`), 201)
}

func extractJSONID(t *testing.T, body []byte) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	id, _ := m["id"].(string)
	if id == "" {
		t.Fatalf("missing id: %s", body)
	}
	return id
}
