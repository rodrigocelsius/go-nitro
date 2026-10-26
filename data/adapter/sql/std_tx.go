package sqladapter

import (
	"context"
	"database/sql"

	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// AdaptStdTx adapts *sql.Tx to gateway.SQLExecutor.
func AdaptStdTx(tx *sql.Tx) gateway.SQLExecutor {
	return &stdTxExec{tx: tx}
}

type stdTxExec struct {
	tx *sql.Tx
}

func (e *stdTxExec) Query(ctx context.Context, query string, args []any) (gateway.Rows, error) {
	rows, err := e.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	cols, err := rows.Columns()
	if err != nil {
		rows.Close()
		return nil, err
	}
	return &stdRows{rows: rows, cols: cols}, nil
}

func (e *stdTxExec) QueryRow(ctx context.Context, query string, args []any) gateway.Row {
	return e.tx.QueryRowContext(ctx, query, args...)
}

func (e *stdTxExec) Exec(ctx context.Context, query string, args []any) (gateway.Result, error) {
	res, err := e.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return stdResult{res: res}, nil
}

// Commit commits the transaction (gateway.TxExecutor).
func (e *stdTxExec) Commit(context.Context) error { return e.tx.Commit() }

// Rollback rolls back the transaction (gateway.TxExecutor).
func (e *stdTxExec) Rollback(context.Context) error { return e.tx.Rollback() }
