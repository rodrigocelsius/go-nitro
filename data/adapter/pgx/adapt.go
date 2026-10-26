package pgx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// AdaptExecutor adapts *Executor to the gateway.SQLExecutor contract.
func AdaptExecutor(e *Executor) gateway.SQLExecutor {
	return &sqlAdapter{exec: e}
}

type sqlAdapter struct {
	exec *Executor
}

func (a *sqlAdapter) Query(ctx context.Context, sql string, args []any) (gateway.Rows, error) {
	rows, err := a.exec.Query(ctx, sql, args)
	if err != nil {
		return nil, err
	}
	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = fd.Name
	}
	return &rowsAdapter{rows: rows, cols: cols}, nil
}

func (a *sqlAdapter) QueryRow(ctx context.Context, sql string, args []any) gateway.Row {
	return a.exec.QueryRow(ctx, sql, args)
}

func (a *sqlAdapter) Exec(ctx context.Context, sql string, args []any) (gateway.Result, error) {
	return a.exec.Exec(ctx, sql, args)
}

type rowsAdapter struct {
	rows pgx.Rows
	cols []string
}

func (r *rowsAdapter) Next() bool             { return r.rows.Next() }
func (r *rowsAdapter) Values() ([]any, error) { return r.rows.Values() }
func (r *rowsAdapter) Columns() []string      { return r.cols }
func (r *rowsAdapter) Err() error             { return r.rows.Err() }
func (r *rowsAdapter) Close()                 { r.rows.Close() }

// BeginTx opens a pool transaction (gateway.TxBeginner → Gateway.InTx).
func (a *sqlAdapter) BeginTx(ctx context.Context) (gateway.TxExecutor, error) {
	tx, err := a.exec.pool.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &txSQLAdapter{tx: tx}, nil
}
