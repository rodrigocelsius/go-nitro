package pgx

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"

	oteladapter "github.com/rodrigocelsius/go-nitro/adapters/otel/v2"
	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// DB is the single bootstrap: pgx pool + Gateway (+ *sql.DB on demand).
//
//	db, err := pgx.Open(ctx, dsn) // NITRO_DB_QUIET=1 disables telemetry
//	defer db.Close()
//	_ = db.MigrateUp(ctx)         // ./migrations (goose)
//	sqlDB := db.SQL()             // GORM / sqlx
//	q := sqlc.New(db.DBTX())      // sqlc
//	store := pgx.StoreOf[T](db, "products")
type DB struct {
	Pool    *Pool
	Gateway *gateway.Gateway

	sqlOnce sync.Once
	sqlDB   *sql.DB
}

// DSN reads DATABASE_URL, then POSTGRES_DSN, or returns fallback.
func DSN(fallback string) string {
	if v := EnvDSN(); v != "" {
		return v
	}
	return fallback
}

// EnvDSN returns DATABASE_URL or POSTGRES_DSN (in that order).
func EnvDSN() string {
	for _, k := range []string{"DATABASE_URL", "POSTGRES_DSN"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// Open opens the pool + Gateway. DB telemetry ON by default (OTel via otel.Ensure).
// If dsn is empty, uses EnvDSN() (DATABASE_URL / POSTGRES_DSN).
// If NITRO_DB_QUIET=1, applies DisableObservability before extra opts.
//
//	db, err := pgx.Open(ctx, "")                    // env only
//	db, err := pgx.Open(ctx, pgx.DSN("postgres://…"))
func Open(ctx context.Context, dsn string, opts ...gateway.Option) (*DB, error) {
	if dsn == "" {
		dsn = EnvDSN()
	}
	if dsn == "" {
		return nil, fmt.Errorf("pgx: missing DSN (set DATABASE_URL or POSTGRES_DSN, or pass dsn to Open)")
	}
	return OpenWithPool(ctx, dsn, PoolConfig{}, opts...)
}

// OpenEnv is a shortcut for Open(ctx, "", opts...).
func OpenEnv(ctx context.Context, opts ...gateway.Option) (*DB, error) {
	return Open(ctx, "", opts...)
}

// OpenWithPool opens a pool with Max/Min conns and SearchPath + Gateway.
//
//	db, err := pgx.OpenWithPool(ctx, dsn, pgx.PoolConfig{
//	    MaxConns: 10, MinConns: 2, SearchPath: "transporte,public",
//	})
func OpenWithPool(ctx context.Context, dsn string, pc PoolConfig, opts ...gateway.Option) (*DB, error) {
	pool, err := NewWithConfig(ctx, dsn, pc)
	if err != nil {
		return nil, err
	}
	// Ensures global OTel Tracer/Meter before Gateway (even if App has not started yet).
	if !oteladapter.Disabled() && os.Getenv("NITRO_DB_QUIET") != "1" {
		_ = oteladapter.Ensure(os.Getenv("OTEL_SERVICE_NAME"))
	}
	gw := gateway.New(AdaptExecutor(NewExecutor(pool)), withEnvQuiet(opts)...)
	return &DB{Pool: pool, Gateway: gw}, nil
}

// OpenGateway is a legacy shortcut: returns pool + gateway (prefer Open).
func OpenGateway(ctx context.Context, dsn string, opts ...gateway.Option) (*Pool, *gateway.Gateway, error) {
	db, err := Open(ctx, dsn, opts...)
	if err != nil {
		return nil, nil, err
	}
	return db.Pool, db.Gateway, nil
}

// Close closes *sql.DB (if opened) and the pool.
func (d *DB) Close() {
	if d == nil {
		return
	}
	if d.sqlDB != nil {
		_ = d.sqlDB.Close()
		d.sqlDB = nil
	}
	d.Pool.Close()
}

// SQL returns *sql.DB observed by Gateway (GORM / sqlx). Lazy + thread-safe.
func (d *DB) SQL() *sql.DB {
	d.sqlOnce.Do(func() {
		d.sqlDB = OpenSQL(d.Pool, d.Gateway)
	})
	return d.sqlDB
}

// DBTX returns the sqlc (pgx) adapter observed by Gateway.
func (d *DB) DBTX() DBTX {
	return AdaptDBTX(d.Gateway, d.Pool)
}

// ExecSQL runs raw SQL on the pool (e.g. schema bootstrap).
func (d *DB) ExecSQL(ctx context.Context, sql string) error {
	if d == nil || d.Pool == nil {
		return fmt.Errorf("pgx: db is nil")
	}
	_, err := d.Pool.DB().Exec(ctx, sql)
	return err
}

// EnsureSchema creates the Postgres schema if missing (no-op for "" / "public").
//
//	_ = db.EnsureSchema(ctx, "transporte")
//	db, err := OpenWithPool(ctx, dsn, PoolConfig{SearchPath: "transporte,public"})
func (d *DB) EnsureSchema(ctx context.Context, schema string) error {
	schema = strings.TrimSpace(schema)
	if schema == "" || strings.EqualFold(schema, "public") {
		return nil
	}
	if !isSafePGIdent(schema) {
		return fmt.Errorf("pgx: EnsureSchema: invalid schema name %q", schema)
	}
	return d.ExecSQL(ctx, fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema))
}

func isSafePGIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' ||
			(i > 0 && r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}
	return true
}

// ExecFS reads name from the FS and runs the SQL (embedded schema.sql).
//
//	//go:embed schema.sql
//	var schemaFS embed.FS
//	_ = db.ExecFS(ctx, schemaFS, "schema.sql")
func (d *DB) ExecFS(ctx context.Context, fsys fs.FS, name string) error {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("pgx: read %s: %w", name, err)
	}
	return d.ExecSQL(ctx, string(b))
}

// StoreOf creates Store[T] reusing this DB's Gateway.
func StoreOf[T any](d *DB, table string, idCol ...string) *Store[T] {
	return NewStore[T](NewDataAPIFromGateway(d.Gateway), table, idCol...)
}

func withEnvQuiet(opts []gateway.Option) []gateway.Option {
	if os.Getenv("NITRO_DB_QUIET") != "1" {
		return opts
	}
	out := make([]gateway.Option, 0, 1+len(opts))
	out = append(out, gateway.DisableObservability())
	return append(out, opts...)
}
