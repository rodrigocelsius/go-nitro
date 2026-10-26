package pgx

import "context"

// InTx runs fn in a transaction on the store's database (domain.Transactor).
// Every Store, StoreRepo or Resource on the same database that is called with
// the ctx passed to fn joins it:
//
//	err := sales.InTx(ctx, func(ctx context.Context) error {
//	    s, err := sales.CreateOne(ctx, &Sale{Total: 10})
//	    if err != nil { return err }
//	    _, err = items.CreateOne(ctx, &SaleItem{SaleID: s.ID, Qty: 1})
//	    return err // non-nil → rollback of both rows
//	})
func (s *Store[T]) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.api.Gateway().InTx(ctx, fn)
}

// InTx implements domain.Transactor (see Store.InTx).
func (r *StoreRepo[T]) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.store.InTx(ctx, fn)
}

// InTx implements domain.Transactor so Resource runs hooks + write atomically.
func (b *storeRepoBridge[T]) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return b.inner.InTx(ctx, fn)
}

// InTx runs fn in a transaction (domain.Transactor). See Store.InTx.
func (d *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return d.Gateway.InTx(ctx, fn)
}
