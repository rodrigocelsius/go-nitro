package sql_test

import (
	"context"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

var User = struct {
	ID    field.Field[int]
	Name  field.Field[string]
	Email field.Field[string]
	Age   field.Field[int]
}{
	ID:    field.Col[int]("users", "id"),
	Name:  field.Col[string]("users", "name"),
	Email: field.Col[string]("users", "email"),
	Age:   field.Col[int]("users", "age"),
}

func compile(q query.Query) (string, []any) {
	c, err := sql.Compile(q.AST(), dialect.Postgres())
	if err != nil {
		panic(err)
	}
	return c.SQL, c.Args
}

func TestCompileSelect_Simple(t *testing.T) {
	q := query.FromName("users").
		Where(User.Name.Eq("Rodrigo")).
		Limit(10)

	got, args := compile(q)
	want := "SELECT * FROM \"users\" WHERE \"name\" = $1 LIMIT 10"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 1 || args[0] != "Rodrigo" {
		t.Fatalf("args: %v", args)
	}
}

func TestCompileSelect_Complex(t *testing.T) {
	q := query.FromName("users").
		Where(User.Age.Gte(18)).
		Where(User.Name.Like("Rod%")).
		OrderBy(User.Name.OrderAsc(), User.ID.OrderDesc()).
		Limit(20).
		Offset(5)

	got, args := compile(q)
	want := "SELECT * FROM \"users\" WHERE \"age\" >= $1 AND \"name\" LIKE $2 ESCAPE '\\' ORDER BY \"name\" ASC, \"id\" DESC LIMIT 20 OFFSET 5"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 2 || args[0] != 18 || args[1] != "Rod%" {
		t.Fatalf("args: %v", args)
	}
}

func TestCompileSelect_InvalidOrderDir(t *testing.T) {
	q := query.FromName("users").OrderBy(expr.Order{
		Field: User.Name.Ref(),
		Dir:   "SIDEWAYS",
	})
	_, err := sql.Compile(q.AST(), dialect.Postgres())
	if err == nil {
		t.Fatal("expected error for invalid Order.Dir")
	}
}

func TestCompileSelect_Or(t *testing.T) {
	q := query.FromName("users").
		Where(User.Age.Lt(18).Or(User.Age.Gte(65)))

	got, args := compile(q)
	want := "SELECT * FROM \"users\" WHERE (\"age\" < $1 OR \"age\" >= $2)"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 2 {
		t.Fatalf("args: %v", args)
	}
}

func TestCompileSelect_Pagination(t *testing.T) {
	q := query.FromName("users").Page(3).PerPage(10)
	got, _ := compile(q)
	want := "SELECT * FROM \"users\" LIMIT 10 OFFSET 20"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompileInsert(t *testing.T) {
	q := query.InsertInto(field.Entity{Name: "users"}).Values(
		User.Name.Set("Ana"),
		User.Email.Set("ana@test.com"),
	)
	got, args := compile(q)
	want := `INSERT INTO "users" ("name", "email") VALUES ($1, $2) RETURNING *`
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 2 || args[0] != "Ana" {
		t.Fatalf("args: %v", args)
	}
}

func TestCompileUpdate(t *testing.T) {
	q := query.Update(field.Entity{Name: "users"}).
		Set(User.Name.Set("Bob")).
		Where(User.ID.Eq(1))
	got, args := compile(q)
	want := `UPDATE "users" SET "name" = $1 WHERE "id" = $2 RETURNING *`
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 2 || args[0] != "Bob" || args[1] != 1 {
		t.Fatalf("args: %v", args)
	}
}

func TestCompileDelete(t *testing.T) {
	q := query.DeleteFrom(field.Entity{Name: "users"}).Where(User.ID.Eq(99))
	got, args := compile(q)
	want := `DELETE FROM "users" WHERE "id" = $1`
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 1 || args[0] != 99 {
		t.Fatalf("args: %v", args)
	}
}

func TestCompileCount(t *testing.T) {
	q := query.FromName("users").Where(User.Age.Gte(18))
	c, err := sql.CompileCount(q.AST(), dialect.Postgres())
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT COUNT(*) FROM "users" WHERE "age" >= $1`
	if c.SQL != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", c.SQL, want)
	}
}

func TestCompileSelect_In(t *testing.T) {
	q := query.FromName("users").Where(User.ID.In(1, 2, 3))
	got, args := compile(q)
	want := `SELECT * FROM "users" WHERE "id" IN ($1, $2, $3)`
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 3 {
		t.Fatalf("args: %v", args)
	}
}

func TestExplain(t *testing.T) {
	p := sql.New(&stubDB{}, dialect.Postgres())
	plan, err := query.FromName("users").Where(User.Name.Eq("x")).Explain(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Text == "" || len(plan.Args) != 1 {
		t.Fatalf("plan: %+v", plan)
	}
}

type stubDB struct{}

func (stubDB) QueryMaps(context.Context, string, []any) ([]map[string]any, error) {
	return nil, nil
}
func (stubDB) Exec(context.Context, string, string, []any) (int64, error) { return 0, nil }
func (stubDB) QueryRowScan(context.Context, string, []any, ...any) error  { return nil }
