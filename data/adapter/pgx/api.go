package pgx

import (
	"context"
	"errors"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	blinqSQL "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// ErrNoRows indicates a SELECT with no results.
var ErrNoRows = errors.New("data: no rows")

// DataAPI is the high-level API for running queries through Gateway
// (logs, metrics, tracing, and optional cache) compiled by BLinq.
type DataAPI struct {
	gw *gateway.Gateway
	p  *blinqSQL.Provider
	d  dialect.Dialect
}

// NewDataAPI creates a DataAPI with Gateway and default observability
// (logger slog ON; tracer/meter no-op; cache OFF).
func NewDataAPI(pool *Pool, opts ...gateway.Option) *DataAPI {
	exec := NewExecutor(pool)
	return NewDataAPIFromGateway(gateway.New(AdaptExecutor(exec), opts...))
}

// NewDataAPIFromExecutor creates DataAPI from an Executor + Gateway options.
func NewDataAPIFromExecutor(executor *Executor, opts ...gateway.Option) *DataAPI {
	return NewDataAPIFromGateway(gateway.New(AdaptExecutor(executor), opts...))
}

// NewDataAPIFromGateway uses an already configured Gateway (PostgreSQL dialect).
func NewDataAPIFromGateway(gw *gateway.Gateway) *DataAPI {
	return NewDataAPIFromGatewayDialect(gw, dialect.Postgres())
}

// NewDataAPIFromGatewayDialect uses Gateway with an explicit SQL dialect
// (MySQL/MariaDB, SQL Server, SQLite — no RETURNING).
func NewDataAPIFromGatewayDialect(gw *gateway.Gateway, d dialect.Dialect) *DataAPI {
	if d == nil {
		d = dialect.Postgres()
	}
	return &DataAPI{
		gw: gw,
		p:  blinqSQL.New(AdaptGateway(gw), d),
		d:  d,
	}
}

// Gateway exposes the Gateway (telemetry / cache).
func (d *DataAPI) Gateway() *gateway.Gateway { return d.gw }

// Dialect returns the SQL dialect used to compile queries.
func (d *DataAPI) Dialect() dialect.Dialect { return d.d }

// Provider returns the BLinq SQL Provider (Explain / Collect).
func (d *DataAPI) Provider() *blinqSQL.Provider { return d.p }

// Select runs a BLinq SELECT and returns rows as maps.
func (d *DataAPI) Select(ctx context.Context, q expr.Query) ([]map[string]any, error) {
	c, err := blinqSQL.Compile(q, d.d)
	if err != nil {
		return nil, fmt.Errorf("data: select: %w", err)
	}
	rows, err := d.gw.Query(ctx, c.SQL, c.Args)
	if err != nil {
		return nil, fmt.Errorf("data: select: %w", err)
	}
	return rows, nil
}

// SelectOne runs SELECT with LIMIT 1.
func (d *DataAPI) SelectOne(ctx context.Context, q expr.Query) (map[string]any, error) {
	one := uint64(1)
	limited := q
	limited.Limit = &one

	result, err := d.Select(ctx, limited)
	if err != nil {
		return nil, fmt.Errorf("data: select one: %w", err)
	}
	if len(result) == 0 {
		return nil, ErrNoRows
	}
	return result[0], nil
}

// SelectWithCount runs SELECT + COUNT for pagination.
func (d *DataAPI) SelectWithCount(ctx context.Context, selectQ, countQ expr.Query) ([]map[string]any, int, error) {
	items, err := d.Select(ctx, selectQ)
	if err != nil {
		return nil, 0, err
	}
	total, err := d.Count(ctx, countQ)
	if err != nil {
		return nil, 0, err
	}
	return items, int(total), nil
}

// Count returns the record count (SELECT COUNT(*)).
func (d *DataAPI) Count(ctx context.Context, q expr.Query) (int64, error) {
	n, err := d.p.CountQuery(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("data: count: %w", err)
	}
	return n, nil
}

// CountQuery counts from the builder.
func (d *DataAPI) CountQuery(ctx context.Context, q query.Query) (int64, error) {
	return d.Count(ctx, q.AST())
}

// Insert runs INSERT (via Exec; RETURNING is ignored by Exec).
func (d *DataAPI) Insert(ctx context.Context, q expr.Query) error {
	c, err := blinqSQL.Compile(q, d.d)
	if err != nil {
		return fmt.Errorf("data: insert: %w", err)
	}
	_, err = d.gw.Exec(ctx, "insert", c.SQL, c.Args)
	if err != nil {
		return fmt.Errorf("data: insert: %w", err)
	}
	return nil
}

// Update runs UPDATE via Exec (no RETURNING — bulk-safe).
func (d *DataAPI) Update(ctx context.Context, q expr.Query) error {
	q.NoReturning = true
	c, err := blinqSQL.Compile(q, d.d)
	if err != nil {
		return fmt.Errorf("data: update: %w", err)
	}
	_, err = d.gw.Exec(ctx, "update", c.SQL, c.Args)
	if err != nil {
		return fmt.Errorf("data: update: %w", err)
	}
	return nil
}

// Delete deletes records and returns how many rows were affected.
func (d *DataAPI) Delete(ctx context.Context, q expr.Query) (int64, error) {
	c, err := blinqSQL.Compile(q, d.d)
	if err != nil {
		return 0, fmt.Errorf("data: delete: %w", err)
	}
	n, err := d.gw.Exec(ctx, "delete", c.SQL, c.Args)
	if err != nil {
		return 0, fmt.Errorf("data: delete: %w", err)
	}
	return n, nil
}

// Raw runs raw SQL (SELECT / RETURNING) via Gateway.Query.
func (d *DataAPI) Raw(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, err := d.gw.Query(ctx, sql, args)
	if err != nil {
		return nil, fmt.Errorf("data: raw query: %w", err)
	}
	return rows, nil
}
