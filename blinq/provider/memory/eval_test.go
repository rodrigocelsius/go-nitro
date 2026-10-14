package memory_test

import (
	"context"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/memory"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestOuterRefCorrelatedExists(t *testing.T) {
	ctx := context.Background()
	db := memory.NewMulti(map[string][]map[string]any{
		"users": {
			{"id": 1, "name": "Ada"},
			{"id": 2, "name": "Bob"},
		},
		"orders": {
			{"id": 10, "user_id": 1, "total": 100},
		},
	})
	userID := field.Col[int]("users", "id")
	orderUser := field.Col[int]("orders", "user_id")

	sub := query.FromName("orders").Where(orderUser.EqOuter(userID.Ref()))
	var got []map[string]any
	err := query.FromName("users").Where(query.Exists(sub)).Collect(ctx, db, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["name"] != "Ada" {
		t.Fatalf("%#v", got)
	}
}

func TestJSONContainsAndPath(t *testing.T) {
	ctx := context.Background()
	meta := field.Col[any]("docs", "meta")
	p := memory.New("docs", []map[string]any{
		{"id": 1, "meta": map[string]any{"a": 1, "b": map[string]any{"c": "x"}}},
		{"id": 2, "meta": map[string]any{"a": 2}},
	})

	var got []map[string]any
	err := query.FromName("docs").
		Where(meta.JSONContains(map[string]any{"a": 1})).
		Collect(ctx, p, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["id"] != 1 {
		t.Fatalf("contains: %#v", got)
	}

	got = nil
	err = query.FromName("docs").
		Where(meta.JSONPathEq("b.c", "x")).
		Collect(ctx, p, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("path: %#v", got)
	}
}

// TestContains_EscapedWildcardsMatchLiterally covers V-34: Contains'
// backslash-escaped "%"/"_" must match the literal characters in the
// in-memory provider too, not act as wildcards.
func TestContains_EscapedWildcardsMatchLiterally(t *testing.T) {
	ctx := context.Background()
	name := field.Col[string]("products", "name")
	p := memory.New("products", []map[string]any{
		{"id": 1, "name": "50% off"},
		{"id": 2, "name": "50X off"},
	})

	var got []map[string]any
	err := query.FromName("products").
		Where(name.Contains("50%")).
		Collect(ctx, p, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["id"] != 1 {
		t.Fatalf("expected only the literal \"50%%\" match, got %#v", got)
	}
}
