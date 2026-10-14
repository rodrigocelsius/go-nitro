package query_test

import (
	"context"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/memory"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

type countingProvider struct {
	provider.Provider
	calls int
}

func (c *countingProvider) Execute(ctx context.Context, q expr.Query, opts ...provider.Option) (provider.Result, error) {
	c.calls++
	return c.Provider.Execute(ctx, q, opts...)
}

func (c *countingProvider) CountQuery(ctx context.Context, q expr.Query) (int64, error) {
	c.calls++
	return c.Provider.(query.Counter).CountQuery(ctx, q)
}

// A short page already tells the exact total: PaginateCollect skips the COUNT
// round-trip there, and still counts when the page is full or past the end.
func TestPaginateCollectSkipsCountOnShortPage(t *testing.T) {
	rows := []map[string]any{}
	for i := 1; i <= 5; i++ {
		rows = append(rows, map[string]any{"id": i})
	}
	id := field.Col[int]("users", "id").OrderAsc()
	cases := []struct {
		page, per            uint64
		wantCalls, wantItems int
	}{
		{1, 10, 1, 5}, // everything fits: no COUNT
		{3, 2, 1, 1},  // last, partial page: no COUNT
		{2, 2, 2, 2},  // full page: COUNT needed
		{9, 2, 2, 0},  // past the end: COUNT needed
	}
	for _, tc := range cases {
		p := &countingProvider{Provider: memory.New("users", rows)}
		got, err := query.PaginateCollect[map[string]any](context.Background(),
			query.FromName("users").OrderBy(id).Paginate(tc.page, tc.per), p)
		if err != nil {
			t.Fatal(err)
		}
		if got.Total != 5 || len(got.Items) != tc.wantItems || p.calls != tc.wantCalls {
			t.Errorf("page %d/%d: total=%d items=%d calls=%d, want total=5 items=%d calls=%d",
				tc.page, tc.per, got.Total, len(got.Items), p.calls, tc.wantItems, tc.wantCalls)
		}
	}
}
