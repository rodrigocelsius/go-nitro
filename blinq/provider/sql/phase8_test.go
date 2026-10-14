package sql_test

import (
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestCompile_ExistsAndInSubquery(t *testing.T) {
	d := dialect.Postgres()
	orderUser := field.Col[int]("orders", "user_id")
	userID := field.Col[int]("users", "id")
	age := field.Col[int]("users", "age")

	sub := query.FromName("orders").
		SelectItems(query.FieldSelect(orderUser.Ref(), "")).
		Where(orderUser.Gt(0))

	q := query.FromName("users").
		Where(age.Gte(18).And(query.InQuery(userID, sub)))

	c, err := sql.Compile(q.AST(), d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.SQL, "IN (SELECT") {
		t.Fatalf("sql=%s", c.SQL)
	}
	if !strings.Contains(c.SQL, "$1") || !strings.Contains(c.SQL, "$2") {
		t.Fatalf("placeholders: %s args=%v", c.SQL, c.Args)
	}

	q2 := query.FromName("users").Where(query.Exists(
		query.FromName("orders").Where(orderUser.Eq(1)),
	))
	c2, err := sql.Compile(q2.AST(), d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c2.SQL, "EXISTS (SELECT") {
		t.Fatalf("%s", c2.SQL)
	}
}
