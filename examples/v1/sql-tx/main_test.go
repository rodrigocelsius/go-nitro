package main

import (
	"context"
	"errors"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func TestTx_ParentChild(t *testing.T) {
	ctx := context.Background()
	userID, err := createUserWithOrder(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if userID.IsZero() {
		t.Fatal("expected user id")
	}
}

func TestTx_Rollback(t *testing.T) {
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ensureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}

	boom := errors.New("boom")
	err = db.Tx(ctx, func(tx *sqladapter.DB) error {
		users := sqladapter.StoreOf[User](tx, "users")
		if _, err := users.CreateOne(ctx, &User{Name: "x"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	users := sqladapter.StoreOf[User](db, "users")
	rows, err := users.Find(ctx, users.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected rollback, got %d", len(rows))
	}
}

func TestTx_CommitCounts(t *testing.T) {
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ensureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}

	var uid nitro.UUID
	err = db.Tx(ctx, func(tx *sqladapter.DB) error {
		users := sqladapter.StoreOf[User](tx, "users")
		orders := sqladapter.StoreOf[Order](tx, "orders")
		u, err := users.CreateOne(ctx, &User{Name: "Bob"})
		if err != nil {
			return err
		}
		uid = u.ID
		_, err = orders.CreateOne(ctx, &Order{UserID: u.ID, Total: 9})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	n, err := countOrdersFor(ctx, db, uid)
	if err != nil || n != 1 {
		t.Fatalf("orders=%d err=%v", n, err)
	}
}
