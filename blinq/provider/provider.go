package provider

import (
	"context"
	"errors"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

var (
	ErrNotFound     = errors.New("blinq: not found")
	ErrUnsupported  = errors.New("blinq: unsupported")
	ErrInvalidQuery = errors.New("blinq: invalid query")
)

// Provider translates the AST and executes it on the backend.
type Provider interface {
	Execute(ctx context.Context, q expr.Query, opts ...Option) (Result, error)
	Explain(ctx context.Context, q expr.Query) (Plan, error)
}

// Result is the typed return via Scan.
type Result interface {
	// ScanSlice fills dest (pointer to a slice of structs or maps).
	ScanSlice(dest any) error
	// ScanOne fills dest (pointer to a struct). ErrNotFound if empty.
	ScanOne(dest any) error
	// Count returns the quantity (aggregate or result-set length).
	Count() (int64, error)
	Err() error
}

// Plan is a readable representation (SQL or in-memory description).
type Plan struct {
	Text string
	Args []any
}

// Option configures execution (not part of the AST).
type Option func(*ExecConfig)

type ExecConfig struct {
	Concurrency int
	Unordered   bool
}

func WithConcurrency(n int) Option {
	return func(c *ExecConfig) { c.Concurrency = n }
}

func WithUnordered() Option {
	return func(c *ExecConfig) { c.Unordered = true }
}

func ApplyOptions(opts []Option) ExecConfig {
	var c ExecConfig
	for _, o := range opts {
		if o != nil {
			o(&c)
		}
	}
	return c
}

// SQLExecutor is the SQL I/O contract — a mirror of data/gateway.SQLExecutor
// so the SQL Provider can run *on* Gateway without the BLinq core importing Nitro.
//
// Uso Nitro:
//
//	gw := gateway.New(pgx.AdaptExecutor(pool))
//	p := sqlprovider.New(gw, dialect.Postgres())
type SQLExecutor interface {
	Query(ctx context.Context, sql string, args []any) (SQLRows, error)
	QueryRow(ctx context.Context, sql string, args []any) SQLRow
	Exec(ctx context.Context, sql string, args []any) (SQLResult, error)
}

type SQLRows interface {
	Next() bool
	Values() ([]any, error)
	Columns() []string
	Err() error
	Close()
}

type SQLRow interface {
	Scan(dest ...any) error
}

type SQLResult interface {
	RowsAffected() int64
}
