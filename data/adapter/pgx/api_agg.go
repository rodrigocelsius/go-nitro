package pgx

import (
	"context"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

// SumColumn sums a column on an arbitrary table (no Store required).
//
//	n, err := api.SumColumn(ctx, "media_objects", "size_bytes")
func (d *DataAPI) SumColumn(ctx context.Context, table, column string) (int64, error) {
	if table == "" || column == "" {
		return 0, fmt.Errorf("pgx: SumColumn requires table and column")
	}
	return query.From(field.Entity{Name: table}).
		SumInt64(ctx, d.Provider(), field.Col[int64](table, column).Ref())
}
