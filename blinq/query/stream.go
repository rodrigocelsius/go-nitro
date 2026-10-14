package query

import (
	"context"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
)

// ForEach iterates SELECT rows without materializing the full typed dest slice
// (still loads maps in the provider; a real cursor is left to dialects with streaming).
func (q Query) ForEach(ctx context.Context, p provider.Provider, fn func(row map[string]any) error, opts ...provider.Option) error {
	rows, err := q.CollectMaps(ctx, p, opts...)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := fn(row); err != nil {
			return err
		}
	}
	return nil
}
