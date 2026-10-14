package provider

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

// ConcurrentCapable is optional — memory/API parallelize; SQL ignores it.
type ConcurrentCapable interface {
	ExecuteConcurrent(ctx context.Context, q expr.Query, workers int, opts ...Option) (Result, error)
}

// Cache is pluggable storage for SELECT result sets.
type Cache interface {
	Get(ctx context.Context, key string) (rows []map[string]any, ok bool)
	Set(ctx context.Context, key string, rows []map[string]any, ttl time.Duration)
	InvalidateTag(ctx context.Context, tag string)
}

// Hook observes the Execute cycle (Before/After).
type Hook interface {
	BeforeExecute(ctx context.Context, q expr.Query) (context.Context, error)
	AfterExecute(ctx context.Context, q expr.Query, res Result, err error)
}

// Telemetry is optional tracing/metrics (nop by default).
type Telemetry interface {
	Start(ctx context.Context, operation string, q expr.Query) (context.Context, func(error))
}

// NopTelemetry does nothing.
type NopTelemetry struct{}

func (NopTelemetry) Start(ctx context.Context, _ string, _ expr.Query) (context.Context, func(error)) {
	return ctx, func(error) {}
}

// FuncHook adapts functions to Hook.
type FuncHook struct {
	Before func(ctx context.Context, q expr.Query) (context.Context, error)
	After  func(ctx context.Context, q expr.Query, res Result, err error)
}

func (h FuncHook) BeforeExecute(ctx context.Context, q expr.Query) (context.Context, error) {
	if h.Before != nil {
		return h.Before(ctx, q)
	}
	return ctx, nil
}

func (h FuncHook) AfterExecute(ctx context.Context, q expr.Query, res Result, err error) {
	if h.After != nil {
		h.After(ctx, q, res, err)
	}
}

// RowsResult exposes raw maps (for cache).
type RowsResult interface {
	Result
	Rows() []map[string]any
}

// Wrap adds hooks, cache, and telemetry around a Provider.
type Wrap struct {
	Inner     Provider
	Cache     Cache
	Hooks     []Hook
	Telemetry Telemetry
	KeyFn     func(ctx context.Context, q expr.Query, p Provider) (string, error)
}

var _ Provider = (*Wrap)(nil)

func (w *Wrap) Explain(ctx context.Context, q expr.Query) (Plan, error) {
	return w.Inner.Explain(ctx, q)
}

func (w *Wrap) Execute(ctx context.Context, q expr.Query, opts ...Option) (res Result, err error) {
	tel := w.Telemetry
	if tel == nil {
		tel = NopTelemetry{}
	}
	ctx, done := tel.Start(ctx, "blinq.execute", q)
	defer func() { done(err) }()

	for _, h := range w.Hooks {
		ctx, err = h.BeforeExecute(ctx, q)
		if err != nil {
			return nil, err
		}
	}
	defer func() {
		for _, h := range w.Hooks {
			h.AfterExecute(ctx, q, res, err)
		}
	}()

	if w.Cache != nil && q.Kind == expr.KindSelect && q.Cache != nil && q.Cache.TTL > 0 {
		if key, kerr := w.cacheKey(ctx, q); kerr == nil {
			if rows, ok := w.Cache.Get(ctx, key); ok {
				res = &cachedResult{rows: rows}
				return res, nil
			}
		}
	}

	cfg := ApplyOptions(opts)
	if cfg.Concurrency > 1 {
		if cc, ok := w.Inner.(ConcurrentCapable); ok {
			res, err = cc.ExecuteConcurrent(ctx, q, cfg.Concurrency, opts...)
		} else {
			res, err = w.Inner.Execute(ctx, q, opts...)
		}
	} else {
		res, err = w.Inner.Execute(ctx, q, opts...)
	}
	if err != nil {
		return res, err
	}

	if w.Cache != nil && q.Kind == expr.KindSelect && q.Cache != nil && q.Cache.TTL > 0 {
		if rr, ok := res.(RowsResult); ok {
			if key, kerr := w.cacheKey(ctx, q); kerr == nil {
				w.Cache.Set(ctx, key, rr.Rows(), q.Cache.TTL)
			}
		}
	}
	return res, err
}

func (w *Wrap) cacheKey(ctx context.Context, q expr.Query) (string, error) {
	if w.KeyFn != nil {
		return w.KeyFn(ctx, q, w.Inner)
	}
	plan, err := w.Inner.Explain(ctx, q)
	if err != nil {
		return "", err
	}
	key := plan.Text
	if q.Cache != nil && q.Cache.Tag != "" {
		key = q.Cache.Tag + "|" + key
	}
	for _, a := range plan.Args {
		key += "|" + fmt.Sprint(a)
	}
	return key, nil
}

type cachedResult struct {
	rows []map[string]any
}

func (r *cachedResult) Err() error             { return nil }
func (r *cachedResult) Count() (int64, error)  { return int64(len(r.rows)), nil }
func (r *cachedResult) Rows() []map[string]any { return r.rows }

func (r *cachedResult) ScanOne(dest any) error {
	if len(r.rows) == 0 {
		return ErrNotFound
	}
	return scan.MapToStruct(r.rows[0], dest)
}

func (r *cachedResult) ScanSlice(dest any) error {
	if dest == nil {
		return fmt.Errorf("blinq: ScanSlice dest nil")
	}
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("blinq: ScanSlice expects *[]T")
	}
	slice := rv.Elem()
	et := slice.Type().Elem()
	if et.Kind() == reflect.Map {
		out := reflect.MakeSlice(slice.Type(), len(r.rows), len(r.rows))
		for i, row := range r.rows {
			out.Index(i).Set(reflect.ValueOf(row))
		}
		slice.Set(out)
		return nil
	}
	out := reflect.MakeSlice(slice.Type(), 0, len(r.rows))
	for _, row := range r.rows {
		elem := reflect.New(et)
		if err := scan.MapToStruct(row, elem.Interface()); err != nil {
			return err
		}
		out = reflect.Append(out, elem.Elem())
	}
	slice.Set(out)
	return nil
}
