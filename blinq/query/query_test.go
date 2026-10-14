package query_test

import (
	"context"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/memory"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

type userFields struct {
	ID      field.Field[int]
	Name    field.Field[string]
	Age     field.Field[int]
	Country field.Field[string]
	Active  field.Field[bool]
}

var (
	UserEntity = field.Entity{Name: "users"}
	user       = userFields{
		ID:      field.Col[int]("users", "id"),
		Name:    field.Col[string]("users", "name"),
		Age:     field.Col[int]("users", "age"),
		Country: field.Col[string]("users", "country"),
		Active:  field.Col[bool]("users", "active"),
	}
)

func sampleProvider(t *testing.T) *memory.Provider {
	t.Helper()
	return memory.New("users", []map[string]any{
		{"id": 1, "name": "Ada", "age": 36, "country": "BR", "active": true},
		{"id": 2, "name": "Bob", "age": 17, "country": "US", "active": true},
		{"id": 3, "name": "Cia", "age": 40, "country": "BR", "active": false},
		{"id": 4, "name": "Dan", "age": 22, "country": "BR", "active": true},
	})
}

func TestWhereAndOr(t *testing.T) {
	ctx := context.Background()
	p := sampleProvider(t)

	var got []map[string]any
	err := query.From(UserEntity).
		Where(user.Age.Gte(18).And(user.Country.Eq("BR"))).
		OrderBy(user.Name.OrderAsc()).
		Collect(ctx, p, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 { // Ada, Cia, Dan
		t.Fatalf("len=%d %#v", len(got), got)
	}

	got = nil
	err = query.From(UserEntity).
		Where(user.Age.Gte(18).And(user.Country.Eq("BR")).And(field.IsTrue(user.Active))).
		Collect(ctx, p, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 { // Ada, Dan
		t.Fatalf("active BR adults: %d", len(got))
	}

	got = nil
	err = query.From(UserEntity).
		Where(user.Country.Eq("US").Or(user.Name.Eq("Ada"))).
		Collect(ctx, p, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("or: %d", len(got))
	}
}

func TestLimitOffsetDistinctFirstCount(t *testing.T) {
	ctx := context.Background()
	p := sampleProvider(t)

	var names []map[string]any
	err := query.From(UserEntity).
		Select(user.Country.Ref()).
		Distinct().
		OrderBy(user.Country.OrderAsc()).
		Collect(ctx, p, &names)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("distinct countries=%d", len(names))
	}

	n, err := query.From(UserEntity).Where(field.IsTrue(user.Active)).Count(ctx, p)
	if err != nil || n != 3 {
		t.Fatalf("count=%d err=%v", n, err)
	}

	var one map[string]any
	err = query.From(UserEntity).
		Where(user.ID.Eq(1)).
		First(ctx, p, &one)
	if err != nil || one["name"] != "Ada" {
		t.Fatalf("first: %#v err=%v", one, err)
	}

	err = query.From(UserEntity).Where(user.ID.Eq(999)).First(ctx, p, &one)
	if err != provider.ErrNotFound {
		t.Fatalf("want ErrNotFound got %v", err)
	}

	var page []map[string]any
	err = query.From(UserEntity).
		OrderBy(user.ID.OrderAsc()).
		Skip(1).Take(2).
		Collect(ctx, p, &page)
	if err != nil || len(page) != 2 || page[0]["id"] != 2 {
		t.Fatalf("page %#v err=%v", page, err)
	}
}

func TestImmutableBuilder(t *testing.T) {
	base := query.From(UserEntity).Where(field.IsTrue(user.Active))
	q2 := base.Where(user.Age.Gte(30))
	if base.AST().Where == nil {
		t.Fatal("base where nil")
	}
	ctx := context.Background()
	p := sampleProvider(t)
	n1, _ := base.Count(ctx, p)
	n2, _ := q2.Count(ctx, p)
	if n1 != 3 || n2 != 1 { // Ada only among active age>=30
		t.Fatalf("immutable broken n1=%d n2=%d", n1, n2)
	}
}

func TestStructScan(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
		Age  int    `db:"age"`
	}
	p, err := memory.NewFromStructs("users", []User{
		{ID: 1, Name: "Ada", Age: 36},
		{ID: 2, Name: "Bob", Age: 17},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out []User
	err = query.From(UserEntity).Where(user.Age.Gte(18)).Collect(context.Background(), p, &out)
	if err != nil || len(out) != 1 || out[0].Name != "Ada" {
		t.Fatalf("%#v err=%v", out, err)
	}
}
