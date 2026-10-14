package provider_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/cache"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/memory"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

type userRow struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

func phase7Rows() []map[string]any {
	rows := make([]map[string]any, 0, 200)
	for i := 1; i <= 200; i++ {
		rows = append(rows, map[string]any{
			"id":   i,
			"name": "u",
			"age":  i % 50,
		})
	}
	return rows
}

func TestExecuteConcurrentOrdered(t *testing.T) {
	ctx := context.Background()
	p := memory.New("users", phase7Rows())
	age := field.Col[int]("users", "age")
	id := field.Col[int]("users", "id")

	var got []map[string]any
	err := query.FromName("users").
		Where(age.Gte(40)).
		OrderBy(id.OrderAsc()).
		Collect(ctx, p, &got, provider.WithConcurrency(8))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected rows")
	}
	for i := 1; i < len(got); i++ {
		prev := got[i-1]["id"].(int)
		cur := got[i]["id"].(int)
		if prev > cur {
			t.Fatalf("unordered at %d: %d > %d", i, prev, cur)
		}
	}
}

func TestExecuteConcurrentUnordered(t *testing.T) {
	ctx := context.Background()
	p := memory.New("users", phase7Rows())
	age := field.Col[int]("users", "age")

	var got []map[string]any
	err := query.FromName("users").
		Where(age.Eq(10)).
		Collect(ctx, p, &got, provider.WithConcurrency(4), provider.WithUnordered())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 { // ids 10,60,110,160
		t.Fatalf("len=%d", len(got))
	}
}

type countingProvider struct {
	inner provider.Provider
	n     *atomic.Int32
}

func (c *countingProvider) Explain(ctx context.Context, q expr.Query) (provider.Plan, error) {
	return c.inner.Explain(ctx, q)
}

func (c *countingProvider) Execute(ctx context.Context, q expr.Query, opts ...provider.Option) (provider.Result, error) {
	c.n.Add(1)
	return c.inner.Execute(ctx, q, opts...)
}

func TestWrapCacheHitMiss(t *testing.T) {
	ctx := context.Background()
	inner := memory.New("users", []map[string]any{
		{"id": 1, "name": "Ada", "age": 36},
		{"id": 2, "name": "Bob", "age": 17},
	})
	mem := cache.New()
	var execs atomic.Int32
	w := &provider.Wrap{
		Inner: &countingProvider{inner: inner, n: &execs},
		Cache: mem,
	}

	q := query.FromName("users").
		Where(field.Col[int]("users", "age").Gte(18)).
		Cache(time.Minute).
		CacheTag("users")

	var first []userRow
	if err := q.Collect(ctx, w, &first); err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Name != "Ada" {
		t.Fatalf("%#v", first)
	}
	if execs.Load() != 1 {
		t.Fatalf("execs=%d", execs.Load())
	}

	var second []userRow
	if err := q.Collect(ctx, w, &second); err != nil {
		t.Fatal(err)
	}
	if execs.Load() != 1 {
		t.Fatalf("cache miss: execs=%d", execs.Load())
	}
	if len(second) != 1 || second[0].Name != "Ada" {
		t.Fatalf("%#v", second)
	}

	mem.InvalidateTag(ctx, "users")
	var third []userRow
	if err := q.Collect(ctx, w, &third); err != nil {
		t.Fatal(err)
	}
	if execs.Load() != 2 {
		t.Fatalf("after invalidate execs=%d", execs.Load())
	}
}

type countingTelemetry struct {
	n *atomic.Int32
}

func (t countingTelemetry) Start(ctx context.Context, _ string, _ expr.Query) (context.Context, func(error)) {
	t.n.Add(1)
	return ctx, func(error) {}
}

func TestWrapHooksAndTelemetry(t *testing.T) {
	ctx := context.Background()
	inner := memory.New("users", []map[string]any{
		{"id": 1, "name": "Ada", "age": 36},
	})
	var before, after, tel atomic.Int32
	w := &provider.Wrap{
		Inner: inner,
		Hooks: []provider.Hook{
			provider.FuncHook{
				Before: func(ctx context.Context, _ expr.Query) (context.Context, error) {
					before.Add(1)
					return ctx, nil
				},
				After: func(_ context.Context, _ expr.Query, _ provider.Result, err error) {
					if err == nil {
						after.Add(1)
					}
				},
			},
		},
		Telemetry: countingTelemetry{n: &tel},
	}

	var got []map[string]any
	err := query.FromName("users").Collect(ctx, w, &got)
	if err != nil {
		t.Fatal(err)
	}
	if before.Load() != 1 || after.Load() != 1 || tel.Load() != 1 {
		t.Fatalf("before=%d after=%d tel=%d", before.Load(), after.Load(), tel.Load())
	}
}

func TestNopTelemetry(t *testing.T) {
	ctx, done := provider.NopTelemetry{}.Start(context.Background(), "x", expr.Query{})
	done(nil)
	if ctx == nil {
		t.Fatal("nil ctx")
	}
}

func TestCacheMemoryTTL(t *testing.T) {
	ctx := context.Background()
	m := cache.New()
	m.Set(ctx, "tag|k", []map[string]any{{"a": 1}}, time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	if _, ok := m.Get(ctx, "tag|k"); ok {
		t.Fatal("expected expired")
	}
}
