package pgx

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
)

func TestIDFromASTInsert(t *testing.T) {
	q := expr.NewInsert(expr.EntityRef{Name: "products"})
	q.Columns = []string{"id", "name"}
	q.Values = []any{"abc", "Widget"}
	got := idFromAST(q, "id")
	if got != "abc" {
		t.Fatalf("got %#v", got)
	}
}

func TestIDFromASTUpdate(t *testing.T) {
	q := expr.NewUpdate(expr.EntityRef{Name: "products"})
	q.Where = expr.Binary{
		Field: expr.FieldRef{Table: "products", Column: "id"},
		Op:    expr.OpEq,
		Value: "xyz",
	}
	got := idFromAST(q, "id")
	if got != "xyz" {
		t.Fatalf("got %#v", got)
	}
}

func TestMySQLDialectNoReturning(t *testing.T) {
	if dialect.MySQL().ReturningSupported() {
		t.Fatal("mysql should not support RETURNING")
	}
	if dialect.MSSQL().ReturningSupported() {
		t.Fatal("mssql should not support RETURNING")
	}
	if dialect.SQLite().ReturningSupported() {
		t.Fatal("sqlite should not support RETURNING")
	}
}
