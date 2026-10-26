package nitro

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Transactor runs a unit of work: every call made with the ctx passed to fn
// joins the same database transaction. Implemented by sqladapter.DB, pgx.DB
// and every BLinq Store.
type Transactor = domain.Transactor

// TxConflictError is a retryable transaction failure (HTTP 409, code "tx_conflict").
type TxConflictError = domain.TxConflictError

// InTx runs fn in a transaction of t (a *sqladapter.DB, a Store, …); with a
// nil t or a repository without transactions fn runs directly. Use it in use
// cases that write several tables:
//
//	err := nitro.InTx(ctx, db, func(ctx context.Context) error {
//	    if err := sales.Create(ctx, sale); err != nil { return err }
//	    return items.CreateMany(ctx, sale.Items) // error → both rolled back
//	})
func InTx(ctx context.Context, t Transactor, fn func(ctx context.Context) error) error {
	return domain.RunInTx(ctx, t, fn)
}

// InTransaction reports whether ctx runs inside a transaction.
func InTransaction(ctx context.Context) bool { return domain.InTx(ctx) }

// AfterCommit runs fn once the transaction in ctx commits (immediately when
// ctx has none). Use it for side effects that must not happen on rollback:
// publishing to a queue, e-mails, webhooks.
//
//	nitro.AfterCommit(ctx, func(ctx context.Context) {
//	    _ = nitro.Publish(ctx, q, "sale.created", sale)
//	})
func AfterCommit(ctx context.Context, fn func(ctx context.Context)) { domain.AfterCommit(ctx, fn) }

// AfterRollback runs fn if the transaction in ctx rolls back (no-op outside one).
func AfterRollback(ctx context.Context, fn func(ctx context.Context)) {
	domain.AfterRollback(ctx, fn)
}

// TxConflict wraps err as a retryable 409 (code "tx_conflict").
func TxConflict(err error) error { return domain.NewTxConflict(err) }

// IsRetryable reports whether err is a transient transaction failure
// (TxConflictError, SQLSTATE 40001 / 40P01) worth retrying.
func IsRetryable(err error) bool { return domain.IsRetryable(err) }
