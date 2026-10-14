package memory

import (
	"context"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
)

// Multi is a multi-table provider (Include / Subquery).
type Multi struct {
	tables map[string][]map[string]any
}

// NewMulti creates a table catalog (key = entity name).
func NewMulti(tables map[string][]map[string]any) *Multi {
	cp := make(map[string][]map[string]any, len(tables))
	for name, rows := range tables {
		out := make([]map[string]any, len(rows))
		for i, r := range rows {
			out[i] = cloneRow(r)
		}
		cp[name] = out
	}
	return &Multi{tables: cp}
}

var _ provider.Provider = (*Multi)(nil)
var _ provider.ConcurrentCapable = (*Multi)(nil)

func (m *Multi) table(name string) ([]map[string]any, error) {
	rows, ok := m.tables[name]
	if !ok {
		return nil, fmt.Errorf("%w: unknown table %q", provider.ErrInvalidQuery, name)
	}
	return rows, nil
}

func (m *Multi) Explain(_ context.Context, q expr.Query) (provider.Plan, error) {
	return provider.Plan{
		Text: fmt.Sprintf("memory-multi:%s kind=%d where=%#v includes=%d",
			q.From.Name, q.Kind, q.Where, len(q.Includes)),
	}, nil
}

func (m *Multi) Execute(ctx context.Context, q expr.Query, opts ...provider.Option) (provider.Result, error) {
	cfg := provider.ApplyOptions(opts)
	if cfg.Concurrency > 1 && q.Kind == expr.KindSelect {
		return m.ExecuteConcurrent(ctx, q, cfg.Concurrency, opts...)
	}
	return m.execute(ctx, q)
}

func (m *Multi) execute(ctx context.Context, q expr.Query) (provider.Result, error) {
	switch q.Kind {
	case expr.KindSelect:
		rows, err := m.selectRows(ctx, q)
		if err != nil {
			return nil, err
		}
		return &result{rows: rows}, nil
	case expr.KindAggregate:
		n, err := m.aggregate(ctx, q)
		if err != nil {
			return nil, err
		}
		return &scalarResult{n: n}, nil
	default:
		return nil, fmt.Errorf("%w: multi memory only Select/Aggregate", provider.ErrUnsupported)
	}
}

func (m *Multi) selectRows(ctx context.Context, q expr.Query) ([]map[string]any, error) {
	src, err := m.table(q.From.Name)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(src))
	ev := &evaluator{resolve: m.resolveRows}
	for _, row := range src {
		ok, err := ev.evalPred(ctx, q.Where, row)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, cloneRow(row))
		}
	}
	return applyOrderLimitProject(out, q), nil
}

func (m *Multi) resolveRows(table string) ([]map[string]any, error) {
	return m.table(table)
}

func (m *Multi) aggregate(ctx context.Context, q expr.Query) (float64, error) {
	if q.Aggregate == nil {
		return 0, fmt.Errorf("blinq/memory: aggregate missing")
	}
	base := q
	base.Kind = expr.KindSelect
	base.Aggregate = nil
	base.Limit = nil
	base.Offset = nil
	base.OrderBy = nil
	rows, err := m.selectRows(ctx, base)
	if err != nil {
		return 0, err
	}
	return aggregateRows(rows, *q.Aggregate)
}

func aggregateRows(rows []map[string]any, agg expr.Aggregate) (float64, error) {
	if agg.Func == expr.AggCount && agg.Field.Column == "" {
		return float64(len(rows)), nil
	}
	vals := make([]float64, 0, len(rows))
	for _, row := range rows {
		v, ok := row[agg.Field.Column]
		if !ok || v == nil {
			continue
		}
		vals = append(vals, toFloat(v))
	}
	if len(vals) == 0 {
		return 0, nil
	}
	switch agg.Func {
	case expr.AggCount:
		return float64(len(vals)), nil
	case expr.AggSum:
		var s float64
		for _, v := range vals {
			s += v
		}
		return s, nil
	case expr.AggAvg:
		var s float64
		for _, v := range vals {
			s += v
		}
		return s / float64(len(vals)), nil
	case expr.AggMin:
		m := vals[0]
		for _, v := range vals[1:] {
			if v < m {
				m = v
			}
		}
		return m, nil
	case expr.AggMax:
		m := vals[0]
		for _, v := range vals[1:] {
			if v > m {
				m = v
			}
		}
		return m, nil
	default:
		return 0, fmt.Errorf("blinq/memory: unknown aggregate %s", agg.Func)
	}
}

// ExecuteConcurrent filters in parallel over the From table.
func (m *Multi) ExecuteConcurrent(ctx context.Context, q expr.Query, workers int, opts ...provider.Option) (provider.Result, error) {
	if q.Kind != expr.KindSelect {
		return m.execute(ctx, q)
	}
	src, err := m.table(q.From.Name)
	if err != nil {
		return nil, err
	}
	// reuses a temporary single-table Provider with resolve
	tmp := &Provider{table: q.From.Name, rows: src, resolve: m.resolveRows}
	return tmp.ExecuteConcurrent(ctx, q, workers, opts...)
}
