package pgx

import (
	"context"
	"fmt"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

// CountAll counts all rows (soft-delete honored via Query).
//
//	n, err := store.CountAll(ctx)
func (s *Store[T]) CountAll(ctx context.Context) (int, error) {
	return s.Count(ctx, s.Query())
}

// CountWhere counts rows matching predicates (AND). Soft-delete honored.
//
//	n, err := store.CountWhere(ctx, modelgen.Tracks.EnrichStatus.Eq("enriched"))
func (s *Store[T]) CountWhere(ctx context.Context, preds ...field.Pred) (int, error) {
	q := s.Query()
	if len(preds) == 1 {
		q = q.Where(preds[0])
	} else if len(preds) > 1 {
		q = q.WhereAll(preds...)
	}
	return s.Count(ctx, q)
}

// Sum runs SUM(col) on the store table (soft-delete honored). No Provider arg needed.
//
//	total, err := store.Sum(ctx, modelgen.Tracks.DurationSeconds.Ref())
func (s *Store[T]) Sum(ctx context.Context, col expr.FieldRef) (float64, error) {
	return s.Query().Sum(ctx, s.api.Provider(), col)
}

// SumInt64 is Sum rounded to int64 (bytes, duration seconds, counters).
func (s *Store[T]) SumInt64(ctx context.Context, col expr.FieldRef) (int64, error) {
	return s.Query().SumInt64(ctx, s.api.Provider(), col)
}

// FindOrdered returns up to limit rows with the given ORDER BY (soft-delete honored).
//
//	recent, err := store.FindOrdered(ctx, 6, modelgen.Albums.CreatedAt.OrderDesc())
func (s *Store[T]) FindOrdered(ctx context.Context, limit int, orders ...expr.Order) ([]T, error) {
	if len(orders) == 0 {
		return nil, fmt.Errorf("pgx: FindOrdered requires at least one Order")
	}
	return s.Find(ctx, s.Query().OrderBy(orders...).Take(limit))
}

// CountBy groups by column and returns COUNT(*) per key, ordered by count DESC.
//
//	rows, err := store.CountBy(ctx, modelgen.Tracks.GenreID.Ref())
func (s *Store[T]) CountBy(ctx context.Context, groupCol expr.FieldRef) ([]query.GroupCount, error) {
	return s.Query().CountGrouped(ctx, s.api.Provider(), groupCol)
}

// CountByTake is CountBy limited to the top n groups.
func (s *Store[T]) CountByTake(ctx context.Context, groupCol expr.FieldRef, n int) ([]query.GroupCount, error) {
	if groupCol.Column == "" {
		return nil, fmt.Errorf("pgx: CountByTake requires group column")
	}
	built := s.Query().
		GroupBy(groupCol).
		SelectItems(
			query.FieldSelect(groupCol, "key"),
			query.AggSelect(expr.Aggregate{Func: expr.AggCount}, "count"),
		).
		OrderBy(expr.Order{Field: expr.FieldRef{Column: "count"}, Dir: expr.OrderDesc}).
		Take(n)

	res, err := built.Execute(ctx, s.api.Provider())
	if err != nil {
		return nil, err
	}
	if mr, ok := res.(interface{ Rows() []map[string]any }); ok {
		return mapsToGroupCounts(mr.Rows()), nil
	}
	type row struct {
		Key   any   `db:"key"`
		Count int64 `db:"count"`
	}
	var scanned []row
	if err := res.ScanSlice(&scanned); err != nil {
		return nil, err
	}
	out := make([]query.GroupCount, 0, len(scanned))
	for _, r := range scanned {
		out = append(out, query.GroupCount{Key: r.Key, Count: r.Count})
	}
	return out, nil
}

// CountBuckets counts rows whose field falls in each half-open [Start, End) bucket.
// Soft-delete is honored. Key is the bucket Label (or RFC3339 Start if Label empty).
//
//	buckets := query.MonthBuckets(6, time.Now())
//	rows, err := store.CountBuckets(ctx, modelgen.Tracks.CreatedAt.Ref(), buckets)
func (s *Store[T]) CountBuckets(ctx context.Context, col expr.FieldRef, buckets []query.TimeBucket) ([]query.GroupCount, error) {
	if col.Column == "" {
		return nil, fmt.Errorf("pgx: CountBuckets requires column")
	}
	out := make([]query.GroupCount, 0, len(buckets))
	f := field.Col[time.Time](col.Table, col.Column)
	for _, b := range buckets {
		n, err := s.CountWhere(ctx, f.InRange(b.Start, b.End))
		if err != nil {
			return out, err
		}
		key := b.Label
		if key == "" {
			key = b.Start.UTC().Format(time.RFC3339)
		}
		out = append(out, query.GroupCount{Key: key, Count: int64(n)})
	}
	return out, nil
}

func mapsToGroupCounts(rows []map[string]any) []query.GroupCount {
	out := make([]query.GroupCount, 0, len(rows))
	for _, m := range rows {
		var key any
		var count any
		for k, v := range m {
			switch k {
			case "key", "Key":
				key = v
			case "count", "Count", "n":
				count = v
			}
		}
		if key == nil {
			// single non-count column is the group key
			for k, v := range m {
				if k != "count" && k != "Count" && k != "n" {
					key = v
					break
				}
			}
		}
		out = append(out, query.GroupCount{Key: key, Count: mapToInt64(count)})
	}
	return out
}

func mapToInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	default:
		return 0
	}
}
