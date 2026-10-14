package pgx

import (
	"context"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	blinqSQL "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/scan"
)

// SelectTyped runs a BLinq SELECT and streams cursor rows directly into []T,
// skipping the intermediate []map[string]any materialization.
//
// Trade-offs vs Select:
//   - driver types preserved end-to-end (no map round-trip);
//   - observability spans wrap acquisition; per-query metrics/logs/row counts
//     and the result cache do not apply.
//
// Use for hot read paths; the default Select stays cached and fully observed.
//
//	items, err := pgx.SelectTyped[Product](ctx, api, query.From(entity).Where(...))
func SelectTyped[T any](ctx context.Context, api *DataAPI, q expr.Query) ([]T, error) {
	c, err := blinqSQL.Compile(q, api.d)
	if err != nil {
		return nil, fmt.Errorf("data: select typed: %w", err)
	}
	rows, err := api.gw.QueryCursor(ctx, c.SQL, c.Args)
	if err != nil {
		return nil, fmt.Errorf("data: select typed: %w", err)
	}
	defer rows.Close()

	var out []T
	if err := scan.ScanCursorInto[T](rows, &out); err != nil {
		return nil, fmt.Errorf("data: select typed: %w", err)
	}
	return out, nil
}
