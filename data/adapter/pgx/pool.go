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
	// pgbouncer is true when the pool talks to PgBouncer (transaction pooling).
	pgbouncer bool
}

// PoolConfig optional settings (in addition to DSN).
type PoolConfig struct {
	MaxConns   int32
	MinConns   int32
	SearchPath string // e.g. "transporte,public"

	// PgBouncer configures the pool for PgBouncer in transaction/statement
	// pooling mode: simple query protocol (no named prepared statements, which
	// PgBouncer < 1.21 cannot route) and no session-level advisory lock for
	// migrations. Also enabled by `pgbouncer=true` in the DSN or
	// NITRO_DB_PGBOUNCER=1. An explicit default_query_exec_mode in the DSN wins.
	PgBouncer bool

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
	dsn, bouncerParam := stripPgBouncerParam(dsn)
	bouncer := pc.PgBouncer || bouncerParam || envBool("NITRO_DB_PGBOUNCER")
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if !bouncer && detectPgBouncerPort(cfg) {
		log.Printf("pgx: port %d is PgBouncer's default — enabling PgBouncer mode (NITRO_DB_PGBOUNCER=0 to disable)", cfg.ConnConfig.Port)
		bouncer = true
	}
	applyPoolEnv(cfg)
	if pc.MaxConns > 0 {
		cfg.MaxConns = pc.MaxConns
	}
	if pc.MinConns > 0 {
		cfg.MinConns = pc.MinConns
	}
	if bouncer {
		configurePgBouncer(cfg, dsn)
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

	return &Pool{pool: pool, pgbouncer: bouncer}, nil
}

// PgBouncer reports whether the pool is configured for PgBouncer.
func (p *Pool) PgBouncer() bool { return p != nil && p.pgbouncer }

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
