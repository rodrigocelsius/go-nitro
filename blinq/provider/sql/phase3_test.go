package sql_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestCompile_SoftSelect(t *testing.T) {
	ent := field.Entity{Name: "users", SoftColumn: "deleted_at"}
	q := query.From(ent).Soft().Where(User.Name.Eq("Ada"))
	got, args := compile(q)
	want := "SELECT * FROM \"users\" WHERE \"name\" = $1 AND \"deleted_at\" IS NULL"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 1 {
		t.Fatalf("args: %v", args)
	}
}

func TestCompile_SoftDelete(t *testing.T) {
	ent := field.Entity{Name: "users", SoftColumn: "deleted_at"}
	q := query.DeleteFrom(ent).Soft().Where(User.ID.Eq(1))
	got, args := compile(q)
	want := "UPDATE \"users\" SET \"deleted_at\" = NOW() WHERE \"id\" = $1"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 1 || args[0] != 1 {
		t.Fatalf("args: %v", args)
	}
}

func TestCompile_Tenant(t *testing.T) {
	ent := field.Entity{Name: "users", TenantColumn: "tenant_id"}
	q := query.From(ent).Tenant(42).Where(User.Age.Gte(18))
	got, args := compile(q)
	want := "SELECT * FROM \"users\" WHERE \"age\" >= $1 AND \"tenant_id\" = $2"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 2 || args[1] != 42 {
		t.Fatalf("args: %v", args)
	}
}

func TestCompile_SoftAndTenant(t *testing.T) {
	ent := field.Entity{Name: "users", SoftColumn: "deleted_at", TenantColumn: "org_id"}
	q := query.From(ent).Soft().Tenant(7)
	got, args := compile(q)
	want := "SELECT * FROM \"users\" WHERE \"deleted_at\" IS NULL AND \"org_id\" = $1"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 1 || args[0] != 7 {
		t.Fatalf("args: %v", args)
	}
}

func TestCompile_AggregateSum(t *testing.T) {
	q := query.FromName("orders").Where(field.Col[int]("orders", "status").Eq(1))
	ast := q.AST()
	ast.Kind = expr.KindAggregate
	ast.Aggregate = &expr.Aggregate{Func: expr.AggSum, Field: expr.FieldRef{Column: "total"}}
	c, err := sql.Compile(ast, dialect.Postgres())
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT SUM(\"total\") FROM \"orders\" WHERE \"status\" = $1"
	if c.SQL != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", c.SQL, want)
	}
}

func TestCompile_InsertTenant(t *testing.T) {
	ent := field.Entity{Name: "users", TenantColumn: "tenant_id"}
	q := query.InsertInto(ent).Tenant(9).Values(User.Name.Set("X"))
	got, args := compile(q)
	want := "INSERT INTO \"users\" (\"name\", \"tenant_id\") VALUES ($1, $2) RETURNING *"
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 2 || args[1] != 9 {
		t.Fatalf("args: %v", args)
	}
}

func TestCompileCount_WithSoft(t *testing.T) {
	ent := field.Entity{Name: "users", SoftColumn: "deleted_at"}
	q := query.From(ent).Soft().Where(User.Age.Gte(18))
	c, err := sql.CompileCount(q.AST(), dialect.Postgres())
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT COUNT(*) FROM \"users\" WHERE \"age\" >= $1 AND \"deleted_at\" IS NULL"
	if c.SQL != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", c.SQL, want)
	}
}
