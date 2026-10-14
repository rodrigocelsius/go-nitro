package memory

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
)

// Provider executes BLinq queries over in-memory slices.
type Provider struct {
	table   string
	rows    []map[string]any
	resolve rowResolver // optional — subqueries / multi
}

// New creates a provider from map rows (key = column name).
func New(table string, rows []map[string]any) *Provider {
	cp := make([]map[string]any, len(rows))
	for i, r := range rows {
		m := make(map[string]any, len(r))
		for k, v := range r {
			m[k] = v
		}
		cp[i] = m
	}
	return &Provider{table: table, rows: cp}
}

// NewFromStructs converts a slice of structs using the `db` tag or field name.
func NewFromStructs(table string, slice any) (*Provider, error) {
	rv := reflect.ValueOf(slice)
	if rv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("blinq/memory: expected slice, got %T", slice)
	}
	rows := make([]map[string]any, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		m, err := structToMap(rv.Index(i))
		if err != nil {
			return nil, err
		}
		rows = append(rows, m)
	}
	return New(table, rows), nil
}

func structToMap(v reflect.Value) (map[string]any, error) {
	v = deref(v)
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("blinq/memory: element not struct")
	}
	t := v.Type()
	m := make(map[string]any, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		name := sf.Name
		if tag := sf.Tag.Get("db"); tag != "" && tag != "-" {
			name = strings.Split(tag, ",")[0]
		}
		m[name] = v.Field(i).Interface()
	}
	return m, nil
}

func deref(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return v
		}
		v = v.Elem()
	}
	return v
}

var _ provider.Provider = (*Provider)(nil)

func (p *Provider) Explain(_ context.Context, q expr.Query) (provider.Plan, error) {
	return provider.Plan{
		Text: fmt.Sprintf("memory:%s kind=%d where=%#v order=%#v limit=%v offset=%v selects=%#v distinct=%v",
			p.table, q.Kind, q.Where, q.OrderBy, q.Limit, q.Offset, q.Selects, q.Distinct),
	}, nil
}

func (p *Provider) Execute(ctx context.Context, q expr.Query, opts ...provider.Option) (provider.Result, error) {
	cfg := provider.ApplyOptions(opts)
	if cfg.Concurrency > 1 && q.Kind == expr.KindSelect {
		return p.ExecuteConcurrent(ctx, q, cfg.Concurrency, opts...)
	}
	switch q.Kind {
	case expr.KindSelect:
		rows, err := p.selectRows(q)
		if err != nil {
			return nil, err
		}
		return &result{rows: rows}, nil
	case expr.KindAggregate:
		n, err := p.AggregateQuery(ctx, q)
		if err != nil {
			return nil, err
		}
		return &scalarResult{n: n}, nil
	default:
		return nil, fmt.Errorf("%w: memory provider only Select/Aggregate (got %d)", provider.ErrUnsupported, q.Kind)
	}
}

func (p *Provider) selectRows(q expr.Query) ([]map[string]any, error) {
	if q.From.Name != "" && p.table != "" && q.From.Name != p.table {
		return nil, fmt.Errorf("%w: table %q != provider %q", provider.ErrInvalidQuery, q.From.Name, p.table)
	}

	ev := &evaluator{resolve: p.resolve}
	out := make([]map[string]any, 0, len(p.rows))
	for _, row := range p.rows {
		ok, err := ev.evalPred(context.Background(), q.Where, row)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, cloneRow(row))
		}
	}
	return applyOrderLimitProject(out, q), nil
}

// CountQuery counts filtered rows (ignores Limit/Offset).
func (p *Provider) CountQuery(_ context.Context, q expr.Query) (int64, error) {
	q.Limit = nil
	q.Offset = nil
	q.OrderBy = nil
	q.Selects = nil
	rows, err := p.selectRows(q)
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

// AggregateQuery evaluates SUM/AVG/MIN/MAX/COUNT in memory.
func (p *Provider) AggregateQuery(_ context.Context, q expr.Query) (float64, error) {
	if q.Aggregate == nil {
		return 0, fmt.Errorf("blinq/memory: aggregate missing")
	}
	q.Limit = nil
	q.Offset = nil
	q.OrderBy = nil
	base := q
	base.Kind = expr.KindSelect
	base.Aggregate = nil
	rows, err := p.selectRows(base)
	if err != nil {
		return 0, err
	}
	agg := *q.Aggregate
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

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return float64(rv.Uint())
		case reflect.Float32, reflect.Float64:
			return rv.Float()
		}
		return 0
	}
}

func cloneRow(r map[string]any) map[string]any {
	m := make(map[string]any, len(r))
	for k, v := range r {
		m[k] = v
	}
	return m
}

type result struct {
	rows []map[string]any
	err  error
}

func (r *result) Err() error { return r.err }

func (r *result) Rows() []map[string]any { return r.rows }

func (r *result) Count() (int64, error) {
	return int64(len(r.rows)), nil
}

func (r *result) ScanSlice(dest any) error {
	dv := reflect.ValueOf(dest)
	if dv.Kind() != reflect.Pointer || dv.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("blinq/memory: ScanSlice expects *[]T")
	}
	slice := dv.Elem()
	elemType := slice.Type().Elem()

	if elemType.Kind() == reflect.Map {
		// *[]map[string]any
		ns := reflect.MakeSlice(slice.Type(), len(r.rows), len(r.rows))
		for i, row := range r.rows {
			ns.Index(i).Set(reflect.ValueOf(row))
		}
		slice.Set(ns)
		return nil
	}

	ns := reflect.MakeSlice(slice.Type(), len(r.rows), len(r.rows))
	for i, row := range r.rows {
		ev := reflect.New(elemType).Elem()
		if err := mapToStruct(row, ev); err != nil {
			return err
		}
		ns.Index(i).Set(ev)
	}
	slice.Set(ns)
	return nil
}

func (r *result) ScanOne(dest any) error {
	if len(r.rows) == 0 {
		return provider.ErrNotFound
	}
	dv := reflect.ValueOf(dest)
	if dv.Kind() != reflect.Pointer {
		return fmt.Errorf("blinq/memory: ScanOne expects pointer")
	}
	return mapToStruct(r.rows[0], dv.Elem())
}

type scalarResult struct{ n float64 }

func (r *scalarResult) Err() error               { return nil }
func (r *scalarResult) Count() (int64, error)    { return int64(r.n), nil }
func (r *scalarResult) ScanSlice(dest any) error { return fmt.Errorf("blinq/memory: scalar") }
func (r *scalarResult) ScanOne(dest any) error   { return fmt.Errorf("blinq/memory: scalar") }

func mapToStruct(row map[string]any, dest reflect.Value) error {
	dest = deref(dest)
	if dest.Kind() == reflect.Map {
		dest.Set(reflect.ValueOf(row))
		return nil
	}
	if dest.Kind() != reflect.Struct {
		return fmt.Errorf("blinq/memory: dest not struct")
	}
	t := dest.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" || !dest.Field(i).CanSet() {
			continue
		}
		name := sf.Name
		if tag := sf.Tag.Get("db"); tag != "" && tag != "-" {
			name = strings.Split(tag, ",")[0]
		}
		val, ok := row[name]
		if !ok {
			// try column without table / case
			for k, v := range row {
				if strings.EqualFold(k, name) {
					val, ok = v, true
					break
				}
			}
		}
		if !ok || val == nil {
			continue
		}
		fv := dest.Field(i)
		rv := reflect.ValueOf(val)
		if rv.Type().AssignableTo(fv.Type()) {
			fv.Set(rv)
			continue
		}
		if rv.Type().ConvertibleTo(fv.Type()) {
			fv.Set(rv.Convert(fv.Type()))
		}
	}
	return nil
}
