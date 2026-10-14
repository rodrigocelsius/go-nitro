// sql-tx — parent + child in one sqladapter.DB.Tx (no Docker).
//
//	cd examples/v1/sql-tx && go test . && go run .
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type User struct {
	nitro.Model
	Name string `db:"name" json:"name"`
}

type Order struct {
	nitro.Model
	UserID nitro.UUID `db:"user_id" json:"user_id"`
	Total  float64    `db:"total" json:"total"`
}

func main() {
	ctx := context.Background()
	userID, err := createUserWithOrder(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("tx ok — user", userID)
}

func createUserWithOrder(ctx context.Context) (nitro.UUID, error) {
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		return nitro.UUID{}, err
	}
	defer db.Close()
	if err := ensureSchema(ctx, db); err != nil {
		return nitro.UUID{}, err
	}

	var userID nitro.UUID
	err = db.Tx(ctx, func(tx *sqladapter.DB) error {
		users := sqladapter.StoreOf[User](tx, "users")
		orders := sqladapter.StoreOf[Order](tx, "orders")
		u, err := users.CreateOne(ctx, &User{Name: "Ada"})
		if err != nil {
			return err
		}
		userID = u.ID
		_, err = orders.CreateOne(ctx, &Order{UserID: u.ID, Total: 42})
		return err
	})
	if err != nil {
		return nitro.UUID{}, err
	}
	return userID, nil
}

func ensureSchema(ctx context.Context, db *sqladapter.DB) error {
	schema := `
CREATE TABLE users (
  id TEXT PRIMARY KEY NOT NULL,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);
CREATE TABLE orders (
  id TEXT PRIMARY KEY NOT NULL,
  user_id TEXT NOT NULL,
  total REAL NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);`
	std := db.StdDB()
	if std == nil {
		return fmt.Errorf("expected sqlite std db")
	}
	_, err := std.ExecContext(ctx, schema)
	return err
}

func countOrdersFor(ctx context.Context, db *sqladapter.DB, userID nitro.UUID) (int, error) {
	orders := sqladapter.StoreOf[Order](db, "orders")
	rows, err := orders.Find(ctx, orders.Query().Where(field.Col[any]("orders", "user_id").Eq(userID)))
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}
