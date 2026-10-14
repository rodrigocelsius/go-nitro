package sqladapter

import (
	"context"
	"database/sql"

	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// AdaptStdDB adapts *sql.DB to gateway.SQLExecutor.
func AdaptStdDB(db *sql.DB) gateway.SQLExecutor {
	return &stdExec{db: db}
}

type stdExec struct {
	db *sql.DB
}

func (e *stdExec) Query(ctx context.Context, query string, args []any) (gateway.Rows, error) {
	rows, err := e.db.QueryContext(ctx, query, args...)
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

func (e *stdExec) QueryRow(ctx context.Context, query string, args []any) gateway.Row {
	return e.db.QueryRowContext(ctx, query, args...)
}

func (e *stdExec) Exec(ctx context.Context, query string, args []any) (gateway.Result, error) {
	res, err := e.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return stdResult{res: res}, nil
}

type stdResult struct {
	res sql.Result
}

func (r stdResult) RowsAffected() int64 {
	n, _ := r.res.RowsAffected()
	return n
}

type stdRows struct {
	rows *sql.Rows
	cols []string
	ptrs []any // Scan scratch, reused across rows
}

func (r *stdRows) Next() bool { return r.rows.Next() }

func (r *stdRows) Values() ([]any, error) {
	// dest is returned to the caller, so it is fresh per row; the pointer
	// slice handed to Scan is only scratch and is reused across rows.
	dest := make([]any, len(r.cols))
	if cap(r.ptrs) < len(r.cols) {
		r.ptrs = make([]any, len(r.cols))
	}
	ptrs := r.ptrs[:len(r.cols)]
	for i := range dest {
		ptrs[i] = &dest[i]
	}
	if err := r.rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	return dest, nil
}

func (r *stdRows) Columns() []string { return r.cols }
func (r *stdRows) Err() error        { return r.rows.Err() }
func (r *stdRows) Close()            { _ = r.rows.Close() }
