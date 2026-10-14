package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
)

var _ provider.ConcurrentCapable = (*Provider)(nil)

type indexedRow struct {
	idx int
	row map[string]any
}

// ExecuteConcurrent filters WHERE in parallel; then Order/Limit sequentially.
// With provider.WithUnordered(), order among chunks is not preserved before OrderBy.
func (p *Provider) ExecuteConcurrent(ctx context.Context, q expr.Query, workers int, opts ...provider.Option) (provider.Result, error) {
	cfg := provider.ApplyOptions(opts)
	if workers < 1 {
		workers = 1
	}
	if q.Kind != expr.KindSelect {
		return p.Execute(ctx, q, opts...)
	}
	if q.From.Name != "" && p.table != "" && q.From.Name != p.table {
		return nil, fmt.Errorf("%w: table %q != provider %q", provider.ErrInvalidQuery, q.From.Name, p.table)
	}

	n := len(p.rows)
	if n == 0 {
		return &result{rows: nil}, nil
	}
	if workers > n {
		workers = n
	}

	var (
		mu      sync.Mutex
		hits    []indexedRow
		errOnce error
	)

	chunk := (n + workers - 1) / workers
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		start := w * chunk
		if start >= n {
			break
		}
		end := start + chunk
		if end > n {
			end = n
		}
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			local := make([]indexedRow, 0, end-start)
			for i := start; i < end; i++ {
				select {
				case <-ctx.Done():
					mu.Lock()
					if errOnce == nil {
						errOnce = ctx.Err()
					}
					mu.Unlock()
					return
				default:
				}
				ok, err := (&evaluator{resolve: p.resolve}).evalPred(ctx, q.Where, p.rows[i])
				if err != nil {
					mu.Lock()
					if errOnce == nil {
						errOnce = err
					}
					mu.Unlock()
					return
				}
				if ok {
					local = append(local, indexedRow{idx: i, row: cloneRow(p.rows[i])})
				}
			}
			mu.Lock()
			hits = append(hits, local...)
			mu.Unlock()
		}(start, end)
	}
	wg.Wait()
	if errOnce != nil {
		return nil, errOnce
	}

	if !cfg.Unordered {
		sort.Slice(hits, func(i, j int) bool { return hits[i].idx < hits[j].idx })
	}

	out := make([]map[string]any, len(hits))
	for i, h := range hits {
		out[i] = h.row
	}
	out = applyOrderLimitProject(out, q)
	return &result{rows: out}, nil
}

func applyOrderLimitProject(out []map[string]any, q expr.Query) []map[string]any {
	if len(q.OrderBy) > 0 {
		sort.SliceStable(out, func(i, j int) bool {
			return lessRows(out[i], out[j], q.OrderBy)
		})
	}
	if q.Distinct {
		out = distinctRows(out, q.Selects)
	}
	offset := uint64(0)
	if q.Offset != nil {
		offset = *q.Offset
	}
	if offset > uint64(len(out)) {
		return nil
	}
	if offset > 0 {
		out = out[offset:]
	}
	if q.Limit != nil && uint64(len(out)) > *q.Limit {
		out = out[:*q.Limit]
	}
	if len(q.Selects) > 0 && !q.Selects[0].All {
		out = project(out, q.Selects)
	}
	return out
}
