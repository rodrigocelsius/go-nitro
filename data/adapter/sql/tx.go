package sqladapter

import (
	"context"
	"fmt"
	"time"
)

// InTx runs fn inside a database transaction (domain.Transactor). Every Store
// built from this DB (StoreFor / StoreOf / RepositoryFor / Resource) that is
// called with the ctx passed to fn joins the transaction — no tx handle to
// thread through. fn returning an error or panicking rolls back; nil commits.
// A nested InTx / Tx on the same DB joins the outer transaction.
//
//	err := db.InTx(ctx, func(ctx context.Context) error {
//	    sale, err := sales.CreateOne(ctx, &Sale{Total: 10})
//	    if err != nil { return err }
//	    _, err = items.CreateOne(ctx, &SaleItem{SaleID: sale.ID})
//	    return err
//	})
//
// Transient failures (serialization / deadlock) come back as
// *domain.TxConflictError (HTTP 409, domain.IsRetryable) — see InTxRetry.
func (d *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if d == nil {
		return fmt.Errorf("sqladapter: Tx on nil DB")
	}
	if fn == nil {
		return fmt.Errorf("sqladapter: Tx requires fn")
	}
	gw := d.Gateway()
	if gw == nil {
		return fmt.Errorf("sqladapter: no pool")
	}
	return gw.InTx(ctx, fn)
}

// InTxRetry is InTx retried up to attempts times (with exponential backoff
// from 50ms) when the transaction fails with a retryable error. fn must be
// safe to re-run: side effects belong in domain.AfterCommit.
func (d *DB) InTxRetry(ctx context.Context, attempts int, fn func(ctx context.Context) error) error {
	return WithRetry(ctx, attempts, 50*time.Millisecond, func(ctx context.Context) error {
		return d.InTx(ctx, fn)
	})
}

// Tx runs fn inside a database transaction, handing it a *DB bound to the
// transaction: StoreOf(tx, …) / StoreFor(tx) use it with any ctx. On success
// the tx is committed; on error or panic it is rolled back. Prefer InTx when
// the code already receives ctx (hooks, use cases).
//
//	err := db.Tx(ctx, func(tx *sqladapter.DB) error {
//	    users := sqladapter.StoreOf[User](tx, "users")
//	    orders := sqladapter.StoreOf[Order](tx, "orders")
//	    u, err := users.CreateOne(ctx, &User{Name: "Ada"})
//	    if err != nil { return err }
//	    _, err = orders.CreateOne(ctx, &Order{UserID: u.ID, Total: 10})
//	    return err
//	})
func (d *DB) Tx(ctx context.Context, fn func(tx *DB) error) error {
	if fn == nil {
		return fmt.Errorf("sqladapter: Tx requires fn")
	}
	return d.InTx(ctx, func(txCtx context.Context) error {
		return fn(&DB{Driver: d.Driver, dialect: d.dialect, gw: d.Gateway().Pin(txCtx)})
	})
}
