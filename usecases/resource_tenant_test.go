package usecases

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

type tenantOrder struct {
	ID       string         `json:"id"`
	TenantID nitrouuid.UUID `json:"tenant_id" db:"tenant_id" readonly:"true"`
	Title    string         `json:"title"`
}

type memTenantRepo struct {
	items map[string]*tenantOrder
}

func (m *memTenantRepo) List(_ context.Context, page, perPage int) (*domain.ResourceListResult, error) {
	out := make([]any, 0, len(m.items))
	for _, it := range m.items {
		cp := *it
		out = append(out, &cp)
	}
	return &domain.ResourceListResult{Items: out, Total: len(out), Page: page, PerPage: perPage}, nil
}

func (m *memTenantRepo) FindByID(_ context.Context, id any) (any, error) {
	it, ok := m.items[fmt.Sprintf("%v", id)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *it
	return &cp, nil
}

func (m *memTenantRepo) Create(_ context.Context, entity any) error {
	o := entity.(*tenantOrder)
	if o.ID == "" {
		o.ID = nitrouuid.NewString()
	}
	cp := *o
	m.items[o.ID] = &cp
	return nil
}

func (m *memTenantRepo) Update(_ context.Context, entity any) error {
	o := entity.(*tenantOrder)
	cp := *o
	m.items[o.ID] = &cp
	return nil
}

func (m *memTenantRepo) Delete(_ context.Context, id any) error {
	delete(m.items, fmt.Sprintf("%v", id))
	return nil
}

func mustUUID(t *testing.T, s string) nitrouuid.UUID {
	t.Helper()
	u, err := nitrouuid.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestTenantScope_GetCrossTenantReturns404(t *testing.T) {
	t1 := mustUUID(t, "01900000-0000-7000-8000-000000000001")
	t2 := mustUUID(t, "01900000-0000-7000-8000-000000000002")
	repo := &memTenantRepo{items: map[string]*tenantOrder{
		"a": {ID: "a", TenantID: t1, Title: "mine"},
		"b": {ID: "b", TenantID: t2, Title: "theirs"},
	}}
	routes := GenerateResourceRoutes("/orders", reflect.TypeOf(tenantOrder{}), repo, ResourceOpts{
		AutoID: true,
		TenantScope: func(*domain.Ctx) (any, bool) {
			return t1.String(), true
		},
	})
	getH := routes[1].Handler

	ctx := &domain.Ctx{AdapterData: &mockJSONResponder{}}
	ctx.SetParam("id", "b")
	err := getH(ctx)
	he, ok := err.(domain.HTTPError)
	if !ok || he.StatusCode() != 404 {
		t.Fatalf("want 404 cross-tenant, got %T %v", err, err)
	}

	ctx2 := &domain.Ctx{AdapterData: &mockJSONResponder{}}
	ctx2.SetParam("id", "a")
	if err := getH(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestTenantScope_ListOnlyOwnTenant(t *testing.T) {
	t1 := mustUUID(t, "01900000-0000-7000-8000-000000000001")
	t2 := mustUUID(t, "01900000-0000-7000-8000-000000000002")
	repo := &memTenantRepo{items: map[string]*tenantOrder{
		"a": {ID: "a", TenantID: t1, Title: "mine"},
		"b": {ID: "b", TenantID: t2, Title: "theirs"},
	}}
	routes := GenerateResourceRoutes("/orders", reflect.TypeOf(tenantOrder{}), repo, ResourceOpts{
		TenantScope: func(*domain.Ctx) (any, bool) {
			return t1.String(), true
		},
	})
	listH := routes[0].Handler
	adapter := &mockJSONResponder{}
	ctx := &domain.Ctx{AdapterData: adapter}
	if err := listH(ctx); err != nil {
		t.Fatal(err)
	}
	res, ok := adapter.body.(*domain.ResourceListResult)
	if !ok {
		t.Fatalf("body type %T", adapter.body)
	}
	if res.Total != 1 || len(res.Items) != 1 {
		t.Fatalf("want 1 item for tenant, got total=%d items=%d", res.Total, len(res.Items))
	}
	got := res.Items[0].(*tenantOrder)
	if got.ID != "a" {
		t.Fatalf("got id %q", got.ID)
	}
}

func TestTenantScope_CreateStampsAndIgnoresClientSpoof(t *testing.T) {
	t1 := mustUUID(t, "01900000-0000-7000-8000-000000000001")
	t2 := mustUUID(t, "01900000-0000-7000-8000-000000000002")
	repo := &memTenantRepo{items: map[string]*tenantOrder{}}
	routes := GenerateResourceRoutes("/orders", reflect.TypeOf(tenantOrder{}), repo, ResourceOpts{
		AutoID: true,
		TenantScope: func(*domain.Ctx) (any, bool) {
			return t1.String(), true
		},
	})
	createH := routes[2].Handler
	adapter := &mockJSONResponder{mockBinder: mockBinder{
		body: []byte(`{"title":"x","tenant_id":"` + t2.String() + `"}`),
	}}
	ctx := &domain.Ctx{AdapterData: adapter}
	if err := createH(ctx); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("items=%d", len(repo.items))
	}
	for _, o := range repo.items {
		if o.TenantID != t1 {
			t.Fatalf("tenant=%s want %s (spoof ignored)", o.TenantID, t1)
		}
		if o.Title != "x" {
			t.Fatalf("title=%q", o.Title)
		}
	}
}

func TestStampEntityTenant_UUID(t *testing.T) {
	o := &tenantOrder{}
	id := mustUUID(t, "01900000-0000-7000-8000-000000000099")
	if err := StampEntityTenant(o, "tenant_id", id.String()); err != nil {
		t.Fatal(err)
	}
	if o.TenantID != id {
		t.Fatalf("got %s", o.TenantID)
	}
}

// Regression: PATCH/PUT must not let a client move a record to another tenant
// by sending tenant_id in the body (mass assignment).
func TestTenantScope_UpdateCannotChangeTenant(t *testing.T) {
	t1 := mustUUID(t, "01900000-0000-7000-8000-000000000001")
	t2 := mustUUID(t, "01900000-0000-7000-8000-000000000002")
	for _, merge := range []bool{true, false} {
		name := "replace"
		if merge {
			name = "merge"
		}
		t.Run(name, func(t *testing.T) {
			repo := &memTenantRepo{items: map[string]*tenantOrder{
				"a": {ID: "a", TenantID: t1, Title: "mine"},
			}}
			routes := GenerateResourceRoutes("/orders", reflect.TypeOf(tenantOrder{}), repo, ResourceOpts{
				AutoID:     true,
				PatchMerge: &merge,
				TenantScope: func(*domain.Ctx) (any, bool) {
					return t1.String(), true
				},
			})
			var updateH domain.Handler
			for _, r := range routes {
				if r.Method == "PATCH" || r.Method == "PUT" {
					updateH = r.Handler
					break
				}
			}
			if updateH == nil {
				t.Fatal("no update route")
			}
			ctx := &domain.Ctx{AdapterData: &mockJSONResponder{mockBinder: mockBinder{
				body: []byte(`{"title":"hacked","tenant_id":"` + t2.String() + `"}`),
			}}}
			ctx.SetParam("id", "a")
			if err := updateH(ctx); err != nil {
				t.Fatal(err)
			}
			got := repo.items["a"]
			if got.TenantID != t1 {
				t.Fatalf("tenant changed to %s (want %s): cross-tenant move via update", got.TenantID, t1)
			}
			if got.Title != "hacked" {
				t.Fatalf("legit field not updated: %q", got.Title)
			}
		})
	}
}
