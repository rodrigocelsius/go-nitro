package pgx

import (
	"context"
	"fmt"
	"strings"

	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// OpenConfig is convention-over-configuration input for OpenFrom.
type OpenConfig struct {
	DSN        string
	Schema     string // EnsureSchema + derived SearchPath when SearchPath empty
	MaxConns   int32
	MinConns   int32
	SearchPath string // optional override (e.g. "transporte,public")
}

// OpenFrom opens the pool using DSN + optional schema/pool settings.
// SearchPath defaults to "schema,public" when Schema is set and not public.
//
//	db, err := pgx.OpenFrom(ctx, pgx.OpenConfig{DSN: dsn, Schema: "transporte", MaxConns: 10})
func OpenFrom(ctx context.Context, cfg OpenConfig, opts ...gateway.Option) (*DB, error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("pgx: OpenFrom: DSN is required")
	}
	pc := PoolConfig{
		MaxConns:   cfg.MaxConns,
		MinConns:   cfg.MinConns,
		SearchPath: cfg.SearchPath,
	}
	if pc.SearchPath == "" {
		pc.SearchPath = searchPathForSchema(cfg.Schema)
	}
	return OpenWithPool(ctx, cfg.DSN, pc, opts...)
}

func searchPathForSchema(schema string) string {
	schema = strings.TrimSpace(schema)
	if schema == "" || strings.EqualFold(schema, "public") {
		return "public"
	}
	return schema + ",public"
}

// Ready runs EnsureSchema (no-op for public) then MigrateUp.
//
//	_ = db.Ready(ctx, cfg.Schema)
func (d *DB) Ready(ctx context.Context, schema string) error {
	if err := d.EnsureSchema(ctx, schema); err != nil {
		return err
	}
	return d.MigrateUp(ctx)
}
