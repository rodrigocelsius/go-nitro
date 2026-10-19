package pgx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	blinqSQL "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

// CountBucketsWhere counts rows per time bucket in a single query, restricted
// by preds (tenant, status, …) and soft delete. Buckets with no rows count 0.
//
//	series, err := users.CountBucketsWhere(ctx, modelgen.Users.CreatedAt.Ref(),
//	    query.DayBuckets(30, time.Now()), modelgen.Users.TenantID.Eq(tid))
//
// Compared with CountBuckets (one COUNT per bucket, no filters) this is one
// round-trip that scans only [first.Start, last.End) — index-friendly on
// (tenant, created_at).
func (s *Store[T]) CountBucketsWhere(ctx context.Context, col expr.FieldRef, buckets []query.TimeBucket, preds ...field.Pred) ([]query.GroupCount, error) {
	if col.Column == "" {
		return nil, fmt.Errorf("pgx: CountBucketsWhere requires column")
	}
	if len(buckets) == 0 {
		return []query.GroupCount{}, nil
	}
	lo, hi := buckets[0].Start, buckets[0].End
	for _, b := range buckets[1:] {
		if b.Start.Before(lo) {
			lo = b.Start
		}
		if b.End.After(hi) {
			hi = b.End
		}
	}
	f := field.Col[time.Time](col.Table, col.Column)
	q := s.Query().Where(f.InRange(lo, hi))
	for _, p := range preds {
		q = q.Where(p)
	}
	d := s.api.Dialect()
	c, err := blinqSQL.CompileCount(q.AST(), d)
	if err != nil {
		return nil, err
	}
	from := strings.Index(c.SQL, " FROM ")
	if from < 0 {
		return nil, fmt.Errorf("pgx: CountBucketsWhere: unexpected count SQL %q", c.SQL)
	}

	// Numbered placeholders ($n, @pn) keep WHERE args first; positional "?"
	// must follow textual order, where the CASE expressions come first.
	positional := d.Placeholder(1) == d.Placeholder(2)
	whereArgs := c.Args
	bucketArgs := make([]any, 0, 2*len(buckets))
	next := len(whereArgs) + 1
	if positional {
		next = 1
	}
	qc := d.QuoteIdent(col.Column)
	var sel strings.Builder
	sel.WriteString("SELECT ")
	for i, b := range buckets {
		if i > 0 {
			sel.WriteString(", ")
		}
		fmt.Fprintf(&sel, "COUNT(CASE WHEN %s >= %s AND %s < %s THEN 1 END) AS %s",
			qc, d.Placeholder(next), qc, d.Placeholder(next+1), d.QuoteIdent(fmt.Sprintf("b%d", i)))
		next += 2
		bucketArgs = append(bucketArgs, b.Start, b.End)
	}
	var args []any
	if positional {
		args = append(bucketArgs, whereArgs...)
	} else {
		args = append(append([]any{}, whereArgs...), bucketArgs...)
	}
	rows, err := s.api.Gateway().Query(ctx, sel.String()+c.SQL[from:], args)
	if err != nil {
		return nil, err
	}
	out := make([]query.GroupCount, len(buckets))
	for i, b := range buckets {
		key := b.Label
		if key == "" {
			key = b.Start.UTC().Format(time.RFC3339)
		}
		var n int64
		if len(rows) > 0 {
			n = toInt64(rows[0][fmt.Sprintf("b%d", i)])
		}
		out[i] = query.GroupCount{Key: key, Count: n}
	}
	return out, nil
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case []byte:
		var n int64
		_, _ = fmt.Sscan(string(x), &n)
		return n
	case string:
		var n int64
		_, _ = fmt.Sscan(x, &n)
		return n
	case fmt.Stringer:
		var n int64
		_, _ = fmt.Sscan(x.String(), &n)
		return n
	default:
		return 0
	}
}
