package query

import (
	"context"
	"fmt"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
)

// GroupCount is one GROUP BY key + COUNT(*) row.
type GroupCount struct {
	Key   any   `json:"key"`
	Count int64 `json:"count"`
}

// TimeBucket is a half-open time window [Start, End) with an optional label.
type TimeBucket struct {
	Start time.Time
	End   time.Time
	Label string
}

// DayBuckets returns n calendar-day buckets ending today (inclusive), oldest
// first, in now's location. Labels are YYYY-MM-DD.
func DayBuckets(n int, now time.Time) []TimeBucket {
	if n < 1 {
		n = 1
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	out := make([]TimeBucket, 0, n)
	for i := n - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i)
		out = append(out, TimeBucket{Start: d, End: d.AddDate(0, 0, 1), Label: d.Format("2006-01-02")})
	}
	return out
}

// HourBuckets returns n one-hour buckets ending at now's hour (inclusive),
// oldest first. Labels are RFC 3339 hour starts.
func HourBuckets(n int, now time.Time) []TimeBucket {
	if n < 1 {
		n = 1
	}
	h := now.Truncate(time.Hour)
	out := make([]TimeBucket, 0, n)
	for i := n - 1; i >= 0; i-- {
		d := h.Add(-time.Duration(i) * time.Hour)
		out = append(out, TimeBucket{Start: d, End: d.Add(time.Hour), Label: d.Format(time.RFC3339)})
	}
	return out
}

// MonthBuckets returns n calendar-month buckets ending at now's month (inclusive), oldest first.
func MonthBuckets(n int, now time.Time) []TimeBucket {
	if n < 1 {
		n = 1
	}
	out := make([]TimeBucket, 0, n)
	for i := n - 1; i >= 0; i-- {
		d := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -i, 0)
		out = append(out, TimeBucket{
			Start: d,
			End:   d.AddDate(0, 1, 0),
			Label: d.Format("2006-01"),
		})
	}
	return out
}

// SumInt64 runs SUM(col) and rounds to int64 (dashboard / byte counters).
func (q Query) SumInt64(ctx context.Context, p provider.Provider, col expr.FieldRef) (int64, error) {
	f, err := q.Sum(ctx, p, col)
	if err != nil {
		return 0, err
	}
	if f < 0 {
		return int64(f - 0.5), nil
	}
	return int64(f + 0.5), nil
}

// CountGrouped runs SELECT groupCol, COUNT(*) … GROUP BY groupCol.
func (q Query) CountGrouped(ctx context.Context, p provider.Provider, groupCol expr.FieldRef) ([]GroupCount, error) {
	if groupCol.Column == "" {
		return nil, fmt.Errorf("blinq: CountGrouped requires group column")
	}
	built := q.
		GroupBy(groupCol).
		SelectItems(
			FieldSelect(groupCol, "key"),
			AggSelect(expr.Aggregate{Func: expr.AggCount}, "count"),
		).
		OrderBy(expr.Order{Field: expr.FieldRef{Column: "count"}, Dir: expr.OrderDesc})

	res, err := built.Execute(ctx, p)
	if err != nil {
		return nil, err
	}
	if mr, ok := res.(interface{ Rows() []map[string]any }); ok {
		return groupCountsFromMaps(mr.Rows()), nil
	}
	type row struct {
		Key   any   `db:"key"`
		Count int64 `db:"count"`
	}
	var scanned []row
	if err := res.ScanSlice(&scanned); err != nil {
		return nil, err
	}
	out := make([]GroupCount, 0, len(scanned))
	for _, r := range scanned {
		out = append(out, GroupCount{Key: r.Key, Count: r.Count})
	}
	return out, nil
}

func groupCountsFromMaps(rows []map[string]any) []GroupCount {
	out := make([]GroupCount, 0, len(rows))
	for _, m := range rows {
		out = append(out, GroupCount{
			Key:   firstMap(m, "key", "Key"),
			Count: toInt64(firstMap(m, "count", "Count", "n")),
		})
	}
	return out
}

func firstMap(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

func toInt64(v any) int64 {
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
	case uint64:
		return int64(n)
	case []byte:
		var x int64
		_, _ = fmt.Sscan(string(n), &x)
		return x
	case string:
		var x int64
		_, _ = fmt.Sscan(n, &x)
		return x
	default:
		return 0
	}
}

// Between is a thin alias of field.Between for call sites that already import query.
func Between[T any](f field.Field[T], lo, hi T) field.Pred {
	return f.Between(lo, hi)
}
