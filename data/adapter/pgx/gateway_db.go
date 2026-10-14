package pgx

import (
	"context"

	blinqSQL "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// gatewayDB adapts *gateway.Gateway to the blinq/provider/sql.DB contract.
type gatewayDB struct {
	gw *gateway.Gateway
}

// AdaptGateway exposes Gateway as the BLinq SQL Provider DB.
func AdaptGateway(gw *gateway.Gateway) blinqSQL.DB {
	return &gatewayDB{gw: gw}
}

func (g *gatewayDB) QueryMaps(ctx context.Context, sql string, args []any) ([]map[string]any, error) {
	return g.gw.Query(ctx, sql, args)
}

func (g *gatewayDB) Exec(ctx context.Context, operation, sql string, args []any) (int64, error) {
	return g.gw.Exec(ctx, operation, sql, args)
}

func (g *gatewayDB) QueryRowScan(ctx context.Context, sql string, args []any, dest ...any) error {
	return g.gw.QueryRow(ctx, sql, args).Scan(dest...)
}
