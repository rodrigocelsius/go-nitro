package query_test

import (
	"context"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/memory"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestPaginateCollect(t *testing.T) {
	ctx := context.Background()
	p := memory.New("users", []map[string]any{
		{"id": 1, "name": "A", "age": 20},
		{"id": 2, "name": "B", "age": 21},
		{"id": 3, "name": "C", "age": 22},
		{"id": 4, "name": "D", "age": 23},
		{"id": 5, "name": "E", "age": 24},
	})

	page, err := query.PaginateCollect[map[string]any](ctx,
		query.FromName("users").OrderBy(field.Col[int]("users", "id").OrderAsc()).Paginate(2, 2),
		p,
	)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 5 {
		t.Fatalf("total=%d", page.Total)
	}
	if page.Page != 2 || page.PerPage != 2 || page.TotalPages != 3 {
		t.Fatalf("page meta: %+v", page)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items=%d", len(page.Items))
	}
	if page.Items[0]["name"] != "C" {
		t.Fatalf("expected C, got %#v", page.Items[0])
	}
}

func TestAggregatesMemory(t *testing.T) {
	ctx := context.Background()
	p := memory.New("orders", []map[string]any{
		{"id": 1, "total": 10.0, "tenant_id": 1},
		{"id": 2, "total": 30.0, "tenant_id": 1},
		{"id": 3, "total": 100.0, "tenant_id": 2},
	})
	ent := field.Entity{Name: "orders", TenantColumn: "tenant_id"}
	totalCol := field.Col[float64]("orders", "total")

	sum, err := query.From(ent).Tenant(1).Sum(ctx, p, totalCol.Ref())
	if err != nil {
		t.Fatal(err)
	}
	if sum != 40 {
		t.Fatalf("sum=%v", sum)
	}
	avg, err := query.From(ent).Tenant(1).Avg(ctx, p, totalCol.Ref())
	if err != nil {
		t.Fatal(err)
	}
	if avg != 20 {
		t.Fatalf("avg=%v", avg)
	}
	n, err := query.From(ent).Tenant(1).Count(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("count=%d", n)
	}
}

func TestSoftTenantSelectMemory(t *testing.T) {
	ctx := context.Background()
	p := memory.New("users", []map[string]any{
		{"id": 1, "name": "Ada", "tenant_id": 1, "deleted_at": nil},
		{"id": 2, "name": "Bob", "tenant_id": 1, "deleted_at": "2020-01-01"},
		{"id": 3, "name": "Cia", "tenant_id": 2, "deleted_at": nil},
	})
	ent := field.Entity{Name: "users", SoftColumn: "deleted_at", TenantColumn: "tenant_id"}

	var got []map[string]any
	err := query.From(ent).Soft().Tenant(1).Collect(ctx, p, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["name"] != "Ada" {
		t.Fatalf("got %#v", got)
	}
}
