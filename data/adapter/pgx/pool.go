package pgx

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps a pgxpool.Pool (PGX connection pool).
type Pool struct {
	pool *pgxpool.Pool
}

// PoolConfig optional settings (in addition to DSN).
type PoolConfig struct {
	MaxConns   int32
	MinConns   int32
	SearchPath string // e.g. "transporte,public"

	// OnNotice receives server NOTICEs. Nil means logNotice, not silence: without a
	// handler pgconn drops them, and a migration that RAISEs a count of the rows it
	// collapsed then reports nothing to anybody.
	OnNotice pgconn.NoticeHandler
}

// New creates a Pool from a DSN (pings).
func New(ctx context.Context, dsn string) (*Pool, error) {
	return NewWithConfig(ctx, dsn, PoolConfig{})
}

// NewWithConfig creates a Pool with optional Max/Min conns and search_path.
func NewWithConfig(ctx context.Context, dsn string, pc PoolConfig) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if pc.MaxConns > 0 {
		cfg.MaxConns = pc.MaxConns
	}
	if pc.MinConns > 0 {
		cfg.MinConns = pc.MinConns
	}
	if pc.SearchPath != "" {
		if cfg.ConnConfig.RuntimeParams == nil {
			cfg.ConnConfig.RuntimeParams = map[string]string{}
		}
		cfg.ConnConfig.RuntimeParams["search_path"] = pc.SearchPath
	}
	cfg.ConnConfig.OnNotice = pc.OnNotice
	if cfg.ConnConfig.OnNotice == nil {
		cfg.ConnConfig.OnNotice = logNotice
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &Pool{pool: pool}, nil
}

// NewFromPool wraps an existing pgxpool.Pool.
func NewFromPool(pool *pgxpool.Pool) *Pool {
	return &Pool{pool: pool}
}

// logNotice is the default NoticeHandler.
//
// A NOTICE is how Postgres reports something that is not an error but is worth
// saying: goose's "relation already exists, skipping" for every IF NOT EXISTS, and
// whatever a migration chooses to RAISE. Dropping them makes a migration
// indistinguishable from one that had nothing to say, which is exactly the case
// where somebody needs to read the output.
func logNotice(_ *pgconn.PgConn, notice *pgconn.Notice) {
	msg := notice.Message
	if notice.Detail != "" {
		msg += " (" + notice.Detail + ")"
	}
	log.Printf("pg notice %s: %s", notice.Severity, msg)
}

// Close closes the pool (no-op if nil).
func (p *Pool) Close() {
	if p == nil || p.pool == nil {
		return
	}
	p.pool.Close()
}

// Ping checks connectivity.
func (p *Pool) Ping(ctx context.Context) error {
	if p == nil || p.pool == nil {
		return fmt.Errorf("pgx: pool is nil")
	}
	return p.pool.Ping(ctx)
}

// DB returns the underlying pgxpool.Pool (advanced use).
func (p *Pool) DB() *pgxpool.Pool {
	if p == nil {
		return nil
	}
	return p.pool
}

// Stat returns pool statistics (nil-safe).
func (p *Pool) Stat() *pgxpool.Stat {
	if p == nil || p.pool == nil {
		return nil
	}
	return p.pool.Stat()
}
