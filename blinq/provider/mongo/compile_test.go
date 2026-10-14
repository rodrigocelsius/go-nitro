package mongo_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/mongo"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestCompileFind(t *testing.T) {
	age := field.Col[int]("users", "age")
	q := query.FromName("users").
		Where(age.Gte(18).And(age.Lt(65))).
		OrderBy(age.OrderDesc()).
		Limit(10)

	f, err := mongo.CompileFind(q.AST())
	if err != nil {
		t.Fatal(err)
	}
	if f.Collection != "users" {
		t.Fatal(f.Collection)
	}
	if f.Filter["$and"] == nil {
		t.Fatalf("%#v", f.Filter)
	}
	if f.Sort["age"] != -1 {
		t.Fatalf("%#v", f.Sort)
	}
	if f.Limit == nil || *f.Limit != 10 {
		t.Fatalf("%v", f.Limit)
	}
}

func TestCompileLookup(t *testing.T) {
	inc := field.HasMany("orders", field.Entity{Name: "orders"},
		field.Col[int]("users", "id").Ref(),
		field.Col[int]("orders", "user_id").Ref(),
	).ToInclude()
	stage := mongo.CompileLookup(inc)
	lookup, ok := stage["$lookup"].(map[string]any)
	if !ok {
		t.Fatalf("%#v", stage)
	}
	if lookup["from"] != "orders" || lookup["localField"] != "id" || lookup["foreignField"] != "user_id" || lookup["as"] != "orders" {
		t.Fatalf("%#v", lookup)
	}
}
