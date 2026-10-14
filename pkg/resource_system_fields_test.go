package nitro

import (
	"encoding/json"
	"strings"
	"testing"
)

type sfProduct struct {
	Model
	Name string `json:"name" db:"name"`
}

type sfOrder struct {
	TenantModel
	Title string `json:"title" db:"title"`
}

func sfDo(t *testing.T, app *App, method, path, body string) (int, map[string]any) {
	t.Helper()
	resp, err := app.Test(method, path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.Unmarshal(resp.Body(), &out)
	return resp.StatusCode(), out
}

// Convention: id/created_at/updated_at/deleted_at are server-owned. A client
// cannot choose the primary key, back-date timestamps or create a row that is
// already soft-deleted (mass assignment, CWE-915).
func TestResource_SystemFieldsAreServerOwned(t *testing.T) {
	app := New(Config{DisableDefaults: true})
	Resource(app, "/products", NewMemoryRepo[sfProduct](), &ResourceConfig{Public: true, NoCache: true})

	const forgedID = "00000000-0000-0000-0000-000000000001"
	code, got := sfDo(t, app, "POST", "/products",
		`{"id":"`+forgedID+`","name":"a","created_at":"2000-01-01T00:00:00Z","updated_at":"2000-01-01T00:00:00Z","deleted_at":"2000-01-01T00:00:00Z"}`)
	if code != 201 {
		t.Fatalf("create status=%d body=%v", code, got)
	}
	id, _ := got["id"].(string)
	if id == "" || id == forgedID {
		t.Fatalf("client chose the id: %v", got["id"])
	}
	for _, k := range []string{"created_at", "updated_at"} {
		if s, _ := got[k].(string); strings.HasPrefix(s, "2000-") {
			t.Fatalf("client back-dated %s: %v", k, s)
		}
	}
	if got["deleted_at"] != nil {
		t.Fatalf("client created a soft-deleted row: %v", got["deleted_at"])
	}
	if code, _ := sfDo(t, app, "GET", "/products/"+id, ""); code != 200 {
		t.Fatalf("created row not visible: %d", code)
	}

	createdAt := got["created_at"]
	code, got = sfDo(t, app, "PATCH", "/products/"+id,
		`{"name":"b","created_at":"2000-01-01T00:00:00Z","deleted_at":"2000-01-01T00:00:00Z"}`)
	if code != 200 {
		t.Fatalf("patch status=%d body=%v", code, got)
	}
	if got["created_at"] != createdAt {
		t.Fatalf("PATCH changed created_at: %v → %v", createdAt, got["created_at"])
	}
	if got["deleted_at"] != nil {
		t.Fatalf("PATCH soft-deleted the row: %v", got["deleted_at"])
	}
	if got["name"] != "b" {
		t.Fatalf("PATCH did not apply name: %v", got["name"])
	}
}

// A tenant must not overwrite another tenant's row by POSTing its id.
func TestResource_CreateCannotHijackOtherTenantRow(t *testing.T) {
	cur := "01900000-0000-7000-8000-00000000000a"
	app := New(Config{DisableDefaults: true})
	setTenant := func(c *Ctx) error { c.SetTenantID(cur); return nil }
	Resource(app, "/orders", NewMemoryRepo[sfOrder](), &ResourceConfig{
		Public: true, NoCache: true, Middleware: []Handler{setTenant},
	})

	_, a := sfDo(t, app, "POST", "/orders", `{"title":"A-secret"}`)
	idA, _ := a["id"].(string)

	cur = "01900000-0000-7000-8000-00000000000b"
	_, b := sfDo(t, app, "POST", "/orders", `{"title":"B","id":"`+idA+`"}`)
	if b["id"] == idA {
		t.Fatalf("tenant B reused tenant A's id: %v", b)
	}

	cur = "01900000-0000-7000-8000-00000000000a"
	code, got := sfDo(t, app, "GET", "/orders/"+idA, "")
	if code != 200 || got["title"] != "A-secret" {
		t.Fatalf("tenant A row was overwritten: %d %v", code, got)
	}
}

// Readonly tags on embedded structs are honored (TenantModel.TenantID with
// NoTenantScope must still not be client-writable).
func TestResource_ReadonlyEmbeddedField(t *testing.T) {
	app := New(Config{DisableDefaults: true})
	Resource(app, "/orders", NewMemoryRepo[sfOrder](), &ResourceConfig{Public: true, NoCache: true, NoTenantScope: true})
	_, got := sfDo(t, app, "POST", "/orders", `{"title":"x","tenant_id":"01900000-0000-7000-8000-00000000000b"}`)
	if tid, _ := got["tenant_id"].(string); tid == "01900000-0000-7000-8000-00000000000b" {
		t.Fatalf("readonly embedded tenant_id was mass-assigned: %v", got)
	}
}

func TestMemoryRepo_CreateDuplicateIDConflicts(t *testing.T) {
	repo := NewMemoryRepo[sfProduct]()
	p := &sfProduct{Name: "a"}
	if err := repo.Create(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	dup := &sfProduct{Model: Model{ID: p.ID}, Name: "b"}
	if err := repo.Create(t.Context(), dup); err == nil {
		t.Fatal("duplicate id must conflict, not overwrite")
	}
	got, _ := repo.FindByID(t.Context(), p.ID.String())
	if got == nil || got.Name != "a" {
		t.Fatalf("original row overwritten: %+v", got)
	}
}
