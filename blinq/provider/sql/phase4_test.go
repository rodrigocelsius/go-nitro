package sql_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestCompile_Join(t *testing.T) {
	users := field.Entity{Name: "users"}
	orders := field.Entity{Name: "orders"}
	uid := field.Col[int]("users", "id")
	oid := field.Col[int]("orders", "user_id")
	name := field.Col[string]("users", "name")
	total := field.Col[float64]("orders", "total")

	q := query.From(users).
		Join(orders, uid.Ref(), oid.Ref()).
		Select(name.Ref(), total.Ref()).
		Where(total.Gte(100))

	got, args := compile(q)
	want := "SELECT \"users\".\"name\", \"orders\".\"total\" FROM \"users\" INNER JOIN \"orders\" ON \"users\".\"id\" = \"orders\".\"user_id\" WHERE \"orders\".\"total\" >= $1"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 1 || args[0] != float64(100) {
		t.Fatalf("args: %v", args)
	}
}

func TestCompile_LeftJoin(t *testing.T) {
	users := field.Entity{Name: "users"}
	orders := field.Entity{Name: "orders"}
	q := query.From(users).
		LeftJoin(orders, field.Col[int]("users", "id").Ref(), field.Col[int]("orders", "user_id").Ref())
	got, _ := compile(q)
	want := "SELECT * FROM \"users\" LEFT JOIN \"orders\" ON \"users\".\"id\" = \"orders\".\"user_id\""
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompile_GroupByHaving(t *testing.T) {
	country := field.Col[string]("users", "country")
	q := query.FromName("users").
		GroupBy(country.Ref()).
		SelectItems(
			query.FieldSelect(country.Ref(), ""),
			query.AggSelect(expr.Aggregate{Func: expr.AggCount}, "n"),
		).
		Having(country.Neq(""))

	got, args := compile(q)
	want := "SELECT \"country\", COUNT(*) AS \"n\" FROM \"users\" GROUP BY \"country\" HAVING \"country\" <> $1"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 1 || args[0] != "" {
		t.Fatalf("args: %v", args)
	}
}

func TestCompile_Upsert(t *testing.T) {
	ent := field.Entity{Name: "users"}
	email := field.Col[string]("users", "email")
	name := field.Col[string]("users", "name")
	q := query.UpsertInto(ent).
		OnConflict(email.Ref()).
		Values(name.Set("Ada"), email.Set("ada@x.com")).
		DoUpdate(name.Set("")) // EXCLUDED.name

	got, args := compile(q)
	want := "INSERT INTO \"users\" (\"name\", \"email\") VALUES ($1, $2) ON CONFLICT (\"email\") DO UPDATE SET \"name\" = EXCLUDED.\"name\" RETURNING *"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 2 {
		t.Fatalf("args: %v", args)
	}
}

func TestCompile_UpsertDoNothing(t *testing.T) {
	ent := field.Entity{Name: "users"}
	email := field.Col[string]("users", "email")
	q := query.UpsertInto(ent).
		OnConflict(email.Ref()).
		DoNothing().
		Values(email.Set("a@b.c"), field.Col[string]("users", "name").Set("X"))

	got, _ := compile(q)
	want := "INSERT INTO \"users\" (\"email\", \"name\") VALUES ($1, $2) ON CONFLICT (\"email\") DO NOTHING RETURNING *"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompile_BulkInsert(t *testing.T) {
	ent := field.Entity{Name: "users"}
	name := field.Col[string]("users", "name")
	q := query.InsertInto(ent).Rows(
		[]expr.Assignment{name.Set("A")},
		[]expr.Assignment{name.Set("B")},
		[]expr.Assignment{name.Set("C")},
	)
	got, args := compile(q)
	want := "INSERT INTO \"users\" (\"name\") VALUES ($1), ($2), ($3) RETURNING *"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 3 {
		t.Fatalf("args: %v", args)
	}
}

func TestCompile_UpdateOmitReturning(t *testing.T) {
	ent := field.Entity{Name: "users"}
	login := field.Col[int]("users", "login_count")
	id := field.Col[int]("users", "id")
	q := query.Update(ent).Set(login.Inc(1)).Where(id.Eq(9)).OmitReturning()
	got, args := compile(q)
	want := "UPDATE \"users\" SET \"login_count\" = \"login_count\" + $1 WHERE \"id\" = $2"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 2 || args[0] != 1 || args[1] != 9 {
		t.Fatalf("args: %v", args)
	}
}

func TestCompile_Inc(t *testing.T) {
	ent := field.Entity{Name: "users"}
	login := field.Col[int]("users", "login_count")
	id := field.Col[int]("users", "id")
	q := query.Update(ent).Set(login.Inc(1)).Where(id.Eq(9))
	got, args := compile(q)
	want := "UPDATE \"users\" SET \"login_count\" = \"login_count\" + $1 WHERE \"id\" = $2 RETURNING *"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 2 || args[0] != 1 || args[1] != 9 {
		t.Fatalf("args: %v", args)
	}
}

func TestBulkChunk_10k(t *testing.T) {
	ent := field.Entity{Name: "items"}
	name := field.Col[string]("items", "name")
	rows := make([][]expr.Assignment, 10000)
	for i := range rows {
		rows[i] = []expr.Assignment{name.Set("x")}
	}

	var queries atomic.Int64
	db := &countingDB{queries: &queries}
	p := sql.New(db, dialect.Postgres())

	_, err := query.InsertInto(ent).Rows(rows...).Chunk(1000).Execute(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if queries.Load() != 10 {
		t.Fatalf("expected 10 chunk queries, got %d", queries.Load())
	}
}

type countingDB struct {
	queries *atomic.Int64
}

func (c *countingDB) QueryMaps(_ context.Context, _ string, args []any) ([]map[string]any, error) {
	c.queries.Add(1)
	return make([]map[string]any, len(args)), nil // 1 arg per row in chunk of 1000
}
func (c *countingDB) Exec(_ context.Context, _, _ string, _ []any) (int64, error) {
	return 0, nil
}
func (c *countingDB) QueryRowScan(context.Context, string, []any, ...any) error { return nil }
