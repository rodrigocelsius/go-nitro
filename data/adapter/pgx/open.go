package pgx

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// OpenStore opens a pool + Store[T] (Phase 9 shortcut).
//
//	store, pool, err := pgx.OpenStore[Product](ctx, dsn, "products")
//	defer pool.Close()
//	app.Resource("/products", &Product{}, pgx.AdaptStore(store), &nitro.ResourceConfig{})
//	app.Resource("/products", &Product{}, pgx.AdaptStore(store), opts)
func OpenStore[T any](ctx context.Context, dsn, table string, opts ...gateway.Option) (*Store[T], *Pool, error) {
	pool, err := New(ctx, dsn)
	if err != nil {
		return nil, nil, err
	}
	return NewStoreFromPool[T](pool, table, opts...), pool, nil
}
