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

func repoFor[T any](db *DB, table string) nitro.TypedRepository[T] {
	store := StoreOf[T](db, table)
	return pgxadapter.NewStoreRepo[T](store)
}
