package graphql_test

import (
	"context"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/graphql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestCompileQuery(t *testing.T) {
	age := field.Col[int]("users", "age")
	q := query.FromName("users").
		Where(age.Gte(18)).
		OrderBy(age.OrderDesc()).
		Limit(10)

	c, err := graphql.CompileQuery(q.AST())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.Query, "query($where: JSON, $limit: Int, $order: JSON)") {
		t.Fatal(c.Query)
	}
	if !strings.Contains(c.Query, "users(where: $where, limit: $limit, order: $order)") {
		t.Fatal(c.Query)
	}
	if !strings.Contains(c.Query, "age") {
		t.Fatal(c.Query)
	}
	where, ok := c.Variables["where"].(map[string]any)
	if !ok || where["age"] == nil {
		t.Fatalf("%#v", c.Variables)
	}
	if c.Variables["limit"] != 10 {
		t.Fatalf("%#v", c.Variables["limit"])
	}
}

func TestCompileQuerySimple(t *testing.T) {
	c, err := graphql.CompileQuery(query.FromName("users").AST())
	if err != nil {
		t.Fatal(err)
	}
	if c.Query != "query { users { id } }" {
		t.Fatal(c.Query)
	}
}

type fakePoster struct {
	lastQuery string
	lastVars  map[string]any
	rows      []map[string]any
}

func (f *fakePoster) PostGraphQL(_ context.Context, query string, variables map[string]any) ([]map[string]any, error) {
	f.lastQuery = query
	f.lastVars = variables
	return f.rows, nil
}

func TestProviderExecute(t *testing.T) {
	fp := &fakePoster{rows: []map[string]any{{"id": 1, "age": 20}}}
	p := &graphql.Provider{Client: fp}
	age := field.Col[int]("users", "age")
	res, err := p.Execute(context.Background(), query.FromName("users").Where(age.Eq(20)).AST())
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	if err := res.ScanSlice(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0]["id"] != 1 {
		t.Fatalf("%#v", out)
	}
	if !strings.Contains(fp.lastQuery, "users(where: $where)") {
		t.Fatal(fp.lastQuery)
	}
}
