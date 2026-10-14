package query

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

// Any returns true if at least one row exists (Limit 1).
func (q Query) Any(ctx context.Context, p provider.Provider, opts ...provider.Option) (bool, error) {
	res, err := q.Limit(1).Execute(ctx, p, opts...)
	if err != nil {
		return false, err
	}
	c, err := res.Count()
	return c > 0, err
}

// All returns true if every row satisfies pred (Count(q) == Count(q.Where(pred))).
func (q Query) All(ctx context.Context, p provider.Provider, pred field.Pred, opts ...provider.Option) (bool, error) {
	total, err := q.Count(ctx, p, opts...)
	if err != nil {
		return false, err
	}
	matched, err := q.Where(pred).Count(ctx, p, opts...)
	if err != nil {
		return false, err
	}
	return total == matched, nil
}

// DryRun is an alias of Explain.
func (q Query) DryRun(ctx context.Context, p provider.Provider) (provider.Plan, error) {
	return q.Explain(ctx, p)
}

// ToSlice is an alias of Collect.
func (q Query) ToSlice(ctx context.Context, p provider.Provider, dest any, opts ...provider.Option) error {
	return q.Collect(ctx, p, dest, opts...)
}

// FirstOrErr is an alias of First.
func (q Query) FirstOrErr(ctx context.Context, p provider.Provider, dest any, opts ...provider.Option) error {
	return q.First(ctx, p, dest, opts...)
}

// After applies a keyset cursor: Where field > value + OrderBy Asc on the field.
func (q Query) After(f expr.FieldRef, value any) Query {
	q = q.clone()
	pred := expr.Binary{Field: f, Op: expr.OpGt, Value: value}
	q.ast.Where = andWhere(q.ast.Where, pred)
	q.ast.OrderBy = append(append([]expr.Order(nil), q.ast.OrderBy...), expr.Order{Field: f, Dir: expr.OrderAsc})
	return q
}

// Before applies a keyset cursor: Where field < value + OrderBy Desc on the field.
func (q Query) Before(f expr.FieldRef, value any) Query {
	q = q.clone()
	pred := expr.Binary{Field: f, Op: expr.OpLt, Value: value}
	q.ast.Where = andWhere(q.ast.Where, pred)
	q.ast.OrderBy = append(append([]expr.Order(nil), q.ast.OrderBy...), expr.Order{Field: f, Dir: expr.OrderDesc})
	return q
}

// RawWhere adds a raw SQL predicate (AND).
func (q Query) RawWhere(sql string, args ...any) Query {
	return q.Where(field.Raw(sql, args...))
}

// CollectTyped deserializes rows into []T.
//
// Without Includes the maps are scanned directly through blinq/scan
// (db tags / snake_case matching; driver types like time.Time preserved —
// no intermediate serialization). With Includes the nested shape falls back
// to a JSON round-trip, matching by json tag.
func CollectTyped[T any](ctx context.Context, q Query, p provider.Provider, opts ...provider.Option) ([]T, error) {
	hasIncludes := len(q.ast.Includes) > 0
	rows, err := q.CollectMaps(ctx, p, opts...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []T{}, nil
	}
	if !hasIncludes {
		out, err := scan.MapsToSlice[T](rows)
		if err != nil {
			return nil, fmt.Errorf("blinq: CollectTyped scan: %w", err)
		}
		return out, nil
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("blinq: CollectTyped marshal: %w", err)
	}
	var out []T
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("blinq: CollectTyped unmarshal: %w", err)
	}
	return out, nil
}

// Into restricts SELECT to the columns and typed Collect (projection).
func Into[T any](ctx context.Context, q Query, p provider.Provider, cols []expr.FieldRef, opts ...provider.Option) ([]T, error) {
	items := make([]expr.SelectItem, len(cols))
	for i, c := range cols {
		items[i] = expr.SelectItem{Field: c}
	}
	var out []T
	if err := q.SelectItems(items...).Collect(ctx, p, &out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}
