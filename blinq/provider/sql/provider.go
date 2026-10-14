package sql

import (
	"context"
	"fmt"
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

// DB is the high-level API used by the SQL Provider.
// Nitro Gateway implements this (Query → maps, Exec → rows affected).
type DB interface {
	QueryMaps(ctx context.Context, sql string, args []any) ([]map[string]any, error)
	Exec(ctx context.Context, operation, sql string, args []any) (int64, error)
	QueryRowScan(ctx context.Context, sql string, args []any, dest ...any) error
}

// ProviderOption configures New.
type ProviderOption func(*Provider)

// WithCache enables SELECT cache with CacheHint (via provider.Wrap).
func WithCache(c provider.Cache) ProviderOption {
	return func(p *Provider) { p.cache = c }
}

// Provider executes BLinq AST via an SQL dialect over DB (typically Gateway).
type Provider struct {
	db    DB
	d     dialect.Dialect
	cache provider.Cache
	exec  provider.Provider // bare or Wrap{Inner: bare, Cache}
}

// bareSQL is the executor without cache (Wrap Inner).
type bareSQL struct {
	db DB
	d  dialect.Dialect
}

// New creates an SQL Provider.
func New(db DB, d dialect.Dialect, opts ...ProviderOption) *Provider {
	if d == nil {
		d = dialect.Postgres()
	}
	p := &Provider{db: db, d: d}
	for _, o := range opts {
		if o != nil {
			o(p)
		}
	}
	bare := &bareSQL{db: p.db, d: p.d}
	if p.cache != nil {
		p.exec = &provider.Wrap{Inner: bare, Cache: p.cache}
	} else {
		p.exec = bare
	}
	return p
}

var _ provider.Provider = (*Provider)(nil)
var _ provider.Provider = (*bareSQL)(nil)

func (p *Provider) Explain(ctx context.Context, q expr.Query) (provider.Plan, error) {
	return p.exec.Explain(ctx, q)
}

func (p *Provider) Execute(ctx context.Context, q expr.Query, opts ...provider.Option) (provider.Result, error) {
	return p.exec.Execute(ctx, q, opts...)
}

func (b *bareSQL) Explain(_ context.Context, q expr.Query) (provider.Plan, error) {
	c, err := Compile(q, b.d)
	if err != nil {
		return provider.Plan{}, err
	}
	return provider.Plan{Text: c.SQL, Args: c.Args}, nil
}

func (b *bareSQL) Execute(ctx context.Context, q expr.Query, _ ...provider.Option) (provider.Result, error) {
	switch q.Kind {
	case expr.KindSelect:
		c, err := Compile(q, b.d)
		if err != nil {
			return nil, err
		}
		rows, err := b.db.QueryMaps(ctx, c.SQL, c.Args)
		if err != nil {
			return nil, err
		}
		return &mapResult{rows: rows}, nil
	case expr.KindAggregate:
		n, err := b.aggregateQuery(ctx, q)
		if err != nil {
			return nil, err
		}
		return &scalarResult{n: n}, nil
	case expr.KindInsert, expr.KindUpdate, expr.KindUpsert:
		c, err := Compile(q, b.d)
		if err != nil {
			return nil, err
		}
		if b.d.ReturningSupported() && sqlHasReturning(c.SQL) {
			rows, err := b.db.QueryMaps(ctx, c.SQL, c.Args)
			if err != nil {
				return nil, err
			}
			return &mapResult{rows: rows}, nil
		}
		op := "insert"
		switch q.Kind {
		case expr.KindUpdate:
			op = "update"
		case expr.KindUpsert:
			op = "upsert"
		}
		n, err := b.db.Exec(ctx, op, c.SQL, c.Args)
		if err != nil {
			return nil, err
		}
		return &execResult{n: n}, nil
	case expr.KindDelete:
		c, err := Compile(q, b.d)
		if err != nil {
			return nil, err
		}
		n, err := b.db.Exec(ctx, "delete", c.SQL, c.Args)
		if err != nil {
			return nil, err
		}
		return &execResult{n: n}, nil
	default:
		return nil, fmt.Errorf("%w: kind %d", provider.ErrUnsupported, q.Kind)
	}
}

// CountQuery runs SELECT COUNT(*).
func (p *Provider) CountQuery(ctx context.Context, q expr.Query) (int64, error) {
	c, err := CompileCount(q, p.d)
	if err != nil {
		return 0, err
	}
	var n int64
	if err := p.db.QueryRowScan(ctx, c.SQL, c.Args, &n); err != nil {
		return 0, err
	}
	return n, nil
}

// AggregateQuery runs SUM/AVG/MIN/MAX/COUNT and returns float64.
func (p *Provider) AggregateQuery(ctx context.Context, q expr.Query) (float64, error) {
	return (&bareSQL{db: p.db, d: p.d}).aggregateQuery(ctx, q)
}

func (b *bareSQL) aggregateQuery(ctx context.Context, q expr.Query) (float64, error) {
	c, err := Compile(q, b.d)
	if err != nil {
		return 0, err
	}
	var n float64
	if err := b.db.QueryRowScan(ctx, c.SQL, c.Args, &n); err != nil {
		return 0, err
	}
	return n, nil
}

type mapResult struct {
	rows []map[string]any
}

func (r *mapResult) Err() error { return nil }
func (r *mapResult) Count() (int64, error) {
	return int64(len(r.rows)), nil
}
func (r *mapResult) ScanSlice(dest any) error {
	return scanIntoSlice(r.rows, dest)
}
func (r *mapResult) ScanOne(dest any) error {
	if len(r.rows) == 0 {
		return provider.ErrNotFound
	}
	return scan.MapToStruct(r.rows[0], dest)
}

// Rows exposes raw maps (Store/DataAPI).
func (r *mapResult) Rows() []map[string]any { return r.rows }

type execResult struct{ n int64 }

func (r *execResult) Err() error               { return nil }
func (r *execResult) Count() (int64, error)    { return r.n, nil }
func (r *execResult) ScanSlice(dest any) error { return fmt.Errorf("blinq/sql: no rows to scan") }
func (r *execResult) ScanOne(dest any) error   { return provider.ErrNotFound }
func (r *execResult) RowsAffected() int64      { return r.n }

type scalarResult struct{ n float64 }

func (r *scalarResult) Err() error            { return nil }
func (r *scalarResult) Count() (int64, error) { return int64(r.n), nil }
func (r *scalarResult) ScanSlice(dest any) error {
	return fmt.Errorf("blinq/sql: scalar result — use AggregateQuery")
}
func (r *scalarResult) ScanOne(dest any) error {
	return fmt.Errorf("blinq/sql: scalar result — use AggregateQuery")
}

func scanIntoSlice(rows []map[string]any, dest any) error {
	return scanMapsToDest(rows, dest)
}

func sqlHasReturning(sql string) bool {
	return strings.Contains(strings.ToUpper(sql), " RETURNING ")
}
