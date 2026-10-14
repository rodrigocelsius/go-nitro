package gateway

import "context"

// SQLExecutor is the I/O contract used by Gateway (implemented by the pgx adapter).
// Kept in package gateway to avoid a circular gateway ↔ pgx dependency.
type SQLExecutor interface {
	Query(ctx context.Context, sql string, args []any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args []any) Row
	Exec(ctx context.Context, sql string, args []any) (Result, error)
}

// Rows represents a SELECT cursor.
type Rows interface {
	Next() bool
	Values() ([]any, error)
	Columns() []string
	Err() error
	Close()
}

// Row represents a single row (QueryRow).
type Row interface {
	Scan(dest ...any) error
}

// Result represents an Exec result.
type Result interface {
	RowsAffected() int64
}
