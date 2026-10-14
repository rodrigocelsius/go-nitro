package pgx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Executor runs compiled queries against the database.
type Executor struct {
	pool *Pool
}

// NewExecutor creates an executor.
func NewExecutor(pool *Pool) *Executor {
	return &Executor{pool: pool}
}

// Query runs SELECT and returns multiple rows.
func (e *Executor) Query(ctx context.Context, sql string, args []any) (pgx.Rows, error) {
	return e.pool.pool.Query(ctx, sql, args...)
}

// QueryRow runs SELECT and returns a single row.
func (e *Executor) QueryRow(ctx context.Context, sql string, args []any) pgx.Row {
	return e.pool.pool.QueryRow(ctx, sql, args...)
}

// Exec runs INSERT/UPDATE/DELETE.
func (e *Executor) Exec(ctx context.Context, sql string, args []any) (pgconn.CommandTag, error) {
	return e.pool.pool.Exec(ctx, sql, args...)
}
