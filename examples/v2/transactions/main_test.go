package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) Publish(_ context.Context, name string, _ any) error {
	r.mu.Lock()
	r.events = append(r.events, name)
	r.mu.Unlock()
	return nil
}

func setup(t *testing.T) (*sqladapter.DB, *nitro.TestClient, *recorder) {
	t.Helper()
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.MigrateFS(ctx, migrations); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	if err := Restock(ctx, db, rec, map[string]int{"A1": 5}); err != nil {
		t.Fatal(err)
	}
	return db, nitro.NewTestClient(newApp(db, rec)), rec
}

func count[T any](t *testing.T, db *sqladapter.DB) int {
	n, err := sqladapter.StoreFor[T](db).CountAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSale_ItemsAndStockInOneTransaction(t *testing.T) {
	db, tc, rec := setup(t)
	resp := tc.Post(t, "/sales", `{"customer":"Ada","items":[{"sku":"A1","qty":2,"price":10},{"sku":"A1","qty":1,"price":10}]}`)
	tc.AssertStatus(t, resp, 201)
	tc.AssertBodyContains(t, resp, `"total":30`)
	if count[SaleItem](t, db) != 2 {
		t.Fatal("items not inserted")
	}
	st, _ := sqladapter.StoreFor[Stock](db).Find(context.Background(), sqladapter.StoreFor[Stock](db).Query())
	if st[0].Qty != 2 {
		t.Fatalf("stock = %d, want 2", st[0].Qty)
	}
	if rec.events[len(rec.events)-1] != "sale.created" {
		t.Fatalf("events = %v", rec.events)
	}
}

func TestSale_RollsBackWhenStockIsShort(t *testing.T) {
	db, tc, rec := setup(t)
	before := len(rec.events)
	resp := tc.Post(t, "/sales", `{"customer":"Bob","items":[{"sku":"A1","qty":2,"price":10},{"sku":"A1","qty":9,"price":10}]}`)
	tc.AssertStatus(t, resp, 409) // the client retries / fixes the order
	if count[Sale](t, db) != 0 || count[SaleItem](t, db) != 0 {
		t.Fatal("sale must be rolled back")
	}
	st, _ := sqladapter.StoreFor[Stock](db).Find(context.Background(), sqladapter.StoreFor[Stock](db).Query())
	if st[0].Qty != 5 {
		t.Fatalf("stock = %d, want 5 (first decrement rolled back)", st[0].Qty)
	}
	if len(rec.events) != before {
		t.Fatal("AfterCommit must not fire on rollback")
	}
	tc.AssertStatus(t, tc.Post(t, "/sales", `{"customer":"Bob","items":[]}`), 422)
}

func TestPayable_PaidClearsCustomer_AndDeleteIsVetoed(t *testing.T) {
	_, tc, _ := setup(t)
	resp := tc.Post(t, "/customers", `{"name":"Ada"}`)
	tc.AssertStatus(t, resp, 201)
	var c Customer
	_ = json.Unmarshal(resp.Body(), &c)
	if c.Status != "pending" {
		t.Fatalf("status = %q", c.Status)
	}

	tc.AssertStatus(t, tc.Post(t, "/payables", `{"customer_id":"`+c.ID.String()+`","amount":50}`), 201)
	tc.AssertStatus(t, tc.Delete(t, "/customers/"+c.ID.String()), 409) // open payable

	tc.AssertStatus(t, tc.Post(t, "/payables", `{"customer_id":"`+c.ID.String()+`","amount":50,"paid":true}`), 201)
	tc.AssertBodyContains(t, tc.Get(t, "/customers/"+c.ID.String()), `"status":"clear"`)

	ghost := nitro.NewUUID().String()
	tc.AssertStatus(t, tc.Post(t, "/payables", `{"customer_id":"`+ghost+`","amount":1,"paid":true}`), 404)
}
