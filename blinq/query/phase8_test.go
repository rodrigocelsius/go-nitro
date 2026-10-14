package query_test

import (
	"context"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/memory"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestIncludeBatch(t *testing.T) {
	ctx := context.Background()
	db := memory.NewMulti(map[string][]map[string]any{
		"users": {
			{"id": 1, "name": "Ada"},
			{"id": 2, "name": "Bob"},
		},
		"orders": {
			{"id": 10, "user_id": 1, "total": 100},
			{"id": 11, "user_id": 1, "total": 50},
			{"id": 12, "user_id": 2, "total": 20},
		},
	})

	users := field.Entity{Name: "users"}
	orders := field.Entity{Name: "orders"}
	userID := field.Col[int]("users", "id")
	orderUser := field.Col[int]("orders", "user_id")
	rel := field.HasMany("orders", orders, userID.Ref(), orderUser.Ref())

	rows, err := query.From(users).Include(rel).CollectMaps(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("users=%d", len(rows))
	}
	adaOrders, _ := rows[0]["orders"].([]map[string]any)
	if rows[0]["name"] != "Ada" {
		// order may vary — find Ada
		for _, r := range rows {
			if r["name"] == "Ada" {
				adaOrders, _ = r["orders"].([]map[string]any)
				break
			}
		}
	}
	if len(adaOrders) != 2 {
		t.Fatalf("ada orders=%d %#v", len(adaOrders), rows)
	}
}

func TestExistsAndInQuery(t *testing.T) {
	ctx := context.Background()
	db := memory.NewMulti(map[string][]map[string]any{
		"users": {
			{"id": 1, "name": "Ada"},
			{"id": 2, "name": "Bob"},
			{"id": 3, "name": "Cia"},
		},
		"orders": {
			{"id": 10, "user_id": 1, "total": 100},
			{"id": 12, "user_id": 2, "total": 20},
		},
	})

	userID := field.Col[int]("users", "id")
	orderUser := field.Col[int]("orders", "user_id")
	orderTotal := field.Col[int]("orders", "total")

	sub := query.FromName("orders").
		SelectItems(query.FieldSelect(orderUser.Ref(), "")).
		Where(orderTotal.Gte(50))

	var got []map[string]any
	err := query.FromName("users").
		Where(query.InQuery(userID, sub)).
		Collect(ctx, db, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["name"] != "Ada" {
		t.Fatalf("%#v", got)
	}

	got = nil
	err = query.FromName("users").
		Where(query.Exists(
			query.FromName("orders").Where(orderUser.Eq(1)),
		)).
		Collect(ctx, db, &got)
	if err != nil {
		t.Fatal(err)
	}
	// uncorrelated EXISTS: true for all users if any order with user_id=1
	if len(got) != 3 {
		t.Fatalf("exists len=%d", len(got))
	}
}

func TestForEach(t *testing.T) {
	ctx := context.Background()
	p := memory.New("users", []map[string]any{
		{"id": 1, "name": "A"},
		{"id": 2, "name": "B"},
	})
	var n int
	err := query.FromName("users").ForEach(ctx, p, func(row map[string]any) error {
		n++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("n=%d", n)
	}
}
