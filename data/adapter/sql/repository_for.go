package sqladapter

import (
	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// RepositoryFor derives the table name (same convention as app.Resource:
// TableName() or plural snake of the type) and returns a ready
// nitro.TypedRepository[T] — the generic equivalent of a typed "db.Sale"
// accessor (Go has no per-model field accessors without code generation).
// Replaces "store := sqladapter.StoreOf[T](db, "table"); repo := pgxadapter.NewStoreRepo(store)".
//
//	repo := sqladapter.RepositoryFor[Sale](db)
func RepositoryFor[T any](db *DB) nitro.TypedRepository[T] {
	return repoFor[T](db, nitro.ResourcePath(new(T)))
}

// StoreFor is StoreOf with the table name taken from the convention shared
// with app.Resource and the CLI (TableName() method, else the snake_case
// plural of the type: Product → products, OrderItem → order_items,
// Person → people). Use StoreOf only for a table outside the convention.
//
//	store := sqladapter.StoreFor[model.Product](db)
func StoreFor[T any](db *DB, idCol ...string) *Store[T] {
	return StoreOf[T](db, nitro.ResourcePath(new(T)), idCol...)
}

func repoFor[T any](db *DB, table string) nitro.TypedRepository[T] {
	store := StoreOf[T](db, table)
	return pgxadapter.NewStoreRepo[T](store)
}
