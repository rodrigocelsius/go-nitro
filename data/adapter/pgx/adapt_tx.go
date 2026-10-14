package pgx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// AdaptTx adapts a pgx transaction to gateway.SQLExecutor (for sqladapter.DB.Tx).
func AdaptTx(tx *Tx) gateway.SQLExecutor {
	if tx == nil {
		return nil
	}
	return &txSQLAdapter{tx: tx.tx}
}

// AdaptPgxTx adapts a raw pgx.Tx.
func AdaptPgxTx(tx pgx.Tx) gateway.SQLExecutor {
	return &txSQLAdapter{tx: tx}
}

type txSQLAdapter struct {
	tx pgx.Tx
}

func (a *txSQLAdapter) Query(ctx context.Context, sql string, args []any) (gateway.Rows, error) {
	rows, err := a.tx.Query(ctx, sql, args...)
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

func (a *txSQLAdapter) QueryRow(ctx context.Context, sql string, args []any) gateway.Row {
	return a.tx.QueryRow(ctx, sql, args...)
}

func (a *txSQLAdapter) Exec(ctx context.Context, sql string, args []any) (gateway.Result, error) {
	tag, err := a.tx.Exec(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return cmdTagResult{tag: tag}, nil
}

type cmdTagResult struct {
	tag pgconn.CommandTag
}

func (r cmdTagResult) RowsAffected() int64 { return r.tag.RowsAffected() }
