package pgx

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Tx wraps a pgx transaction.
type Tx struct {
	tx pgx.Tx
}

// Begin starts a transaction.
func (e *Executor) Begin(ctx context.Context) (*Tx, error) {
	tx, err := e.pool.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx}, nil
}

// Commit commits the transaction.
func (tx *Tx) Commit(ctx context.Context) error {
	return tx.tx.Commit(ctx)
}

// Rollback rolls back the transaction.
func (tx *Tx) Rollback(ctx context.Context) error {
	return tx.tx.Rollback(ctx)
}

// Query runs SELECT inside the transaction.
func (tx *Tx) Query(ctx context.Context, sql string, args []any) (pgx.Rows, error) {
	return tx.tx.Query(ctx, sql, args...)
}

// QueryRow runs a single-row SELECT inside the transaction.
func (tx *Tx) QueryRow(ctx context.Context, sql string, args []any) pgx.Row {
	return tx.tx.QueryRow(ctx, sql, args...)
}

// Exec runs INSERT/UPDATE/DELETE inside the transaction.
func (tx *Tx) Exec(ctx context.Context, sql string, args []any) error {
	_, err := tx.tx.Exec(ctx, sql, args...)
	return err
}
