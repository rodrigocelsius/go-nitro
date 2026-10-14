package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// testExec returns one user row until a write statement runs, then none.
type testExec struct{ writes int }

func (e *testExec) Query(_ context.Context, _ string, _ []any) (Rows, error) {
	if e.writes > 0 {
		return &testRows{}, nil
	}
	return &testRows{n: 1}, nil
}

func (e *testExec) QueryRow(context.Context, string, []any) Row { return testRow{} }

func (e *testExec) Exec(_ context.Context, sql string, _ []any) (Result, error) {
	e.writes++
	return testResult(len(sql)), nil
}

type testRows struct{ n int }

func (r *testRows) Next() bool { return r.n > 0 }
func (r *testRows) Values() ([]any, error) {
	r.n--
	return []any{"u1"}, nil
}
func (r *testRows) Columns() []string { return []string{"id"} }
func (r *testRows) Err() error        { return nil }
func (r *testRows) Close()            {}

type testRow struct{}

func (testRow) Scan(...any) error { return nil }

type testResult int

func (r testResult) RowsAffected() int64 { return int64(r) }

func TestExtractTable_Insert(t *testing.T) {
	tests := []struct {
		sql  string
		want string
	}{
		{"INSERT INTO users (name, email) VALUES ($1, $2)", "users"},
		{"insert into orders (total) values ($1)", "orders"},
		{"INSERT INTO  products(name) VALUES($1)", "products"},
	}
	for _, tt := range tests {
		got := extractTable(tt.sql)
		if got != tt.want {
			t.Errorf("extractTable(%q) = %q, want %q", tt.sql, got, tt.want)
		}
	}
}

func TestExtractTable_Update(t *testing.T) {
	tests := []struct {
		sql  string
		want string
	}{
		{"UPDATE users SET name=$1 WHERE id=$2", "users"},
		{"update orders set total=$1 where id=$2", "orders"},
	}
	for _, tt := range tests {
		got := extractTable(tt.sql)
		if got != tt.want {
			t.Errorf("extractTable(%q) = %q, want %q", tt.sql, got, tt.want)
		}
	}
}

func TestExtractTable_Delete(t *testing.T) {
	tests := []struct {
		sql  string
		want string
	}{
		{"DELETE FROM users WHERE id=$1", "users"},
		{"delete from orders where id=$1", "orders"},
	}
	for _, tt := range tests {
		got := extractTable(tt.sql)
		if got != tt.want {
			t.Errorf("extractTable(%q) = %q, want %q", tt.sql, got, tt.want)
		}
	}
}

func TestExtractTable_Select(t *testing.T) {
	got := extractTable("SELECT * FROM users WHERE id=$1")
	if got != "" {
		t.Errorf("extractTable(SELECT...) should return empty, got %q", got)
	}
}

func TestCacheKey_Deterministic(t *testing.T) {
	k1 := cacheKey("select", "SELECT * FROM users", []any{1})
	k2 := cacheKey("select", "SELECT * FROM users", []any{1})
	if k1 != k2 {
		t.Errorf("cacheKey should be deterministic: %q != %q", k1, k2)
	}
}

func TestCacheKey_DiffersByOperation(t *testing.T) {
	k1 := cacheKey("select", "SELECT * FROM users", nil)
	k2 := cacheKey("insert", "SELECT * FROM users", nil)
	if k1 == k2 {
		t.Error("cacheKey should differ by operation")
	}
}

func TestCacheKey_DiffersBySQL(t *testing.T) {
	k1 := cacheKey("select", "SELECT * FROM users", nil)
	k2 := cacheKey("select", "SELECT * FROM orders", nil)
	if k1 == k2 {
		t.Error("cacheKey should differ by SQL")
	}
}

func TestCacheKey_DiffersByArgs(t *testing.T) {
	k1 := cacheKey("select", "SELECT * FROM users WHERE id=$1", []any{1})
	k2 := cacheKey("select", "SELECT * FROM users WHERE id=$1", []any{2})
	if k1 == k2 {
		t.Error("cacheKey should differ by args")
	}
}

func TestNew_DefaultsEnabled(t *testing.T) {
	gw := New(&stubExec{})
	if !gw.LoggerEnabled() || !gw.TracerEnabled() || !gw.MeterEnabled() {
		t.Fatal("defaults should enable logger/tracer/meter")
	}
	if gw.CacheEnabled() {
		t.Fatal("cache should be off by default")
	}
}

func TestNew_DisableObservability(t *testing.T) {
	gw := New(&stubExec{}, DisableObservability())
	if gw.LoggerEnabled() || gw.TracerEnabled() || gw.MeterEnabled() || gw.CacheEnabled() {
		t.Fatal("DisableObservability should turn everything off")
	}
}

func TestNew_DisableLoggerOnly(t *testing.T) {
	gw := New(&stubExec{}, DisableLogger())
	if gw.LoggerEnabled() {
		t.Fatal("logger should be off")
	}
	if !gw.TracerEnabled() || !gw.MeterEnabled() {
		t.Fatal("tracer/meter should remain on")
	}
}

type stubExec struct{}

func (stubExec) Query(context.Context, string, []any) (Rows, error) { return nil, nil }
func (stubExec) QueryRow(context.Context, string, []any) Row        { return stubRow{} }
func (stubExec) Exec(context.Context, string, []any) (Result, error) {
	return stubResult{}, nil
}

type stubRow struct{}

func (stubRow) Scan(...any) error { return nil }

type stubResult struct{}

func (stubResult) RowsAffected() int64 { return 0 }

func TestInvalidateBySQL_RoundTrip(t *testing.T) {
	// Regression: SELECT keys embed the table; a quoted UPDATE must
	// invalidate them. Before the fix the pattern kept the quotes and
	// never matched, so stale rows were served for 5 minutes.
	mem := newTestCache()
	exec := &testExec{}
	g := New(exec, WithCache(mem))

	sql := `SELECT * FROM "users" WHERE id = $1`
	first, _ := g.Query(context.Background(), sql, []any{1})
	if len(first) != 1 {
		t.Fatalf("expected 1 row from executor, got %d", len(first))
	}
	cached, _ := g.Query(context.Background(), sql, []any{1})
	if len(cached) != 1 { // second call may come from cache; shape identical
		t.Fatalf("unexpected second read: %d", len(cached))
	}
	if len(mem.items) != 1 {
		t.Fatalf("expected select to be cached, cache=%v", mem.items)
	}

	if _, err := g.Exec(context.Background(), "update", `UPDATE "users" SET name = $1 WHERE id = $2`, []any{"x", 1}); err != nil {
		t.Fatal(err)
	}
	if len(mem.items) != 0 {
		t.Fatalf("expected invalidation on write, cache=%v", mem.items)
	}

	// After the write the SELECT must re-execute against the (mutated) executor.
	after, _ := g.Query(context.Background(), sql, []any{1})
	if len(after) != 0 {
		t.Fatalf("stale cache served after update: %v", after)
	}
}

// testCache is a minimal domain.Cache with prefix-pattern Invalidate
// (same semantics as the memory adapter: trailing '*' = prefix match).
type testCache struct{ items map[string]any }

func newTestCache() *testCache { return &testCache{items: map[string]any{}} }

func (c *testCache) Get(key string, dest any) error {
	v, ok := c.items[key]
	if !ok {
		return errors.New("not found")
	}
	p, ok := dest.(*[]map[string]any)
	if !ok {
		return errors.New("bad dest")
	}
	rows := v.([]map[string]any)
	cp := make([]map[string]any, len(rows))
	copy(cp, rows)
	*p = cp
	return nil
}

func (c *testCache) Set(key string, value any, _ time.Duration) error {
	c.items[key] = value
	return nil
}

func (c *testCache) Delete(key string) error { delete(c.items, key); return nil }

func (c *testCache) Invalidate(pattern string) error {
	for k := range c.items {
		if pattern != "" && strings.HasSuffix(pattern, "*") && strings.HasPrefix(k, pattern[:len(pattern)-1]) {
			delete(c.items, k)
		}
	}
	return nil
}

func (c *testCache) Clear() error { c.items = map[string]any{}; return nil }

func TestSelectCacheTable(t *testing.T) {
	tests := []struct {
		sql  string
		want string
	}{
		{`SELECT * FROM users WHERE id = $1`, "users"},
		{`SELECT * FROM "users" WHERE id = $1`, "users"},
		{"SELECT * FROM `users`", "users"},
		{"SELECT * FROM [users]", "users"},
		{`SELECT u.* FROM public.users u LIMIT 10`, "public.users"},
		{`WITH t AS (SELECT 1) SELECT * FROM users`, "users"},
	}
	for _, tt := range tests {
		if got := selectCacheTable(tt.sql); got != tt.want {
			t.Errorf("selectCacheTable(%q) = %q, want %q", tt.sql, got, tt.want)
		}
	}
	for _, sql := range []string{
		`SELECT * FROM users JOIN orders ON orders.user_id = users.id`,
		`SELECT * FROM users, orders`,
		`SELECT * FROM (SELECT id FROM users) sub`,
		`INSERT INTO users (id) VALUES ($1)`,
	} {
		if got := selectCacheTable(sql); got != "" {
			t.Errorf("selectCacheTable(%q) = %q, want empty", sql, got)
		}
	}
}

func TestExtractTable_QuotedAndMerge(t *testing.T) {
	tests := []struct{ sql, want string }{
		{`UPDATE "users" SET name=$1 WHERE id=$2`, "users"},
		{"UPDATE `orders` SET total=$1", "orders"},
		{`DELETE FROM "users" WHERE id=$1`, "users"},
		{`MERGE INTO [products] WITH (HOLDLOCK) AS t USING (VALUES ($1)) AS s(id) ON t.id=s.id WHEN MATCHED THEN UPDATE SET name=$2;`, "products"},
		{`merge into products using (select $1) s on products.id = s.id when matched then update set name = $2`, "products"},
	}
	for _, tt := range tests {
		if got := extractTable(tt.sql); got != tt.want {
			t.Errorf("extractTable(%q) = %q, want %q", tt.sql, got, tt.want)
		}
	}
}
