package sqladapter

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	oteladapter "github.com/rodrigocelsius/go-nitro/adapters/otel/v2"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// Store is the typed BLinq store (same as pgx.Store).
type Store[T any] = pgxadapter.Store[T]

// DB is a dialect-aware SQL handle: Postgres uses pgx; other engines use database/sql.
type DB struct {
	Driver  Driver
	pg      *pgxadapter.DB
	std     *sql.DB
	gw      *gateway.Gateway
	dialect dialect.Dialect
}

// EnvDSN returns DATABASE_URL, then DB_DSN, then POSTGRES_DSN.
func EnvDSN() string {
	for _, k := range []string{"DATABASE_URL", "DB_DSN", "POSTGRES_DSN"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// EnvDriver returns NITRO_DB_DRIVER (postgresql|mysql|mariadb|sqlserver|sqlite).
func EnvDriver() string {
	return os.Getenv("NITRO_DB_DRIVER")
}

// OpenEnv opens from NITRO_DB_DRIVER + DATABASE_URL / DB_DSN.
func OpenEnv(ctx context.Context, opts ...gateway.Option) (*DB, error) {
	return Open(ctx, EnvDriver(), EnvDSN(), opts...)
}

// Open opens a pool for driver+dsn. Empty driver is inferred from the DSN.
func Open(ctx context.Context, driver, dsn string, opts ...gateway.Option) (*DB, error) {
	if dsn == "" {
		dsn = EnvDSN()
	}
	d := detectDriver(driver, dsn)
	if d == DriverPostgres {
		if dsn == "" {
			return nil, errMissingDSN()
		}
		pg, err := pgxadapter.Open(ctx, dsn, opts...)
		if err != nil {
			return nil, err
		}
		return &DB{Driver: DriverPostgres, pg: pg, dialect: dialect.Postgres()}, nil
	}
	if dsn == "" {
		if d == DriverSQLite {
			dsn = "file:data/app.db"
		} else {
			return nil, errMissingDSN()
		}
	}
	name := stdDriverName(d)
	if name == "" {
		return nil, fmt.Errorf("sqladapter: unknown driver %q", driver)
	}
	if d == DriverSQLite {
		if err := ensureSQLiteDir(dsn); err != nil {
			return nil, err
		}
		dsn = sqliteDSNDefaults(dsn)
	}
	if !slices.Contains(sql.Drivers(), name) {
		return nil, fmt.Errorf("sqladapter: driver %q is not compiled in (built with -tags nitro_no%s?)", name, map[string]string{"sqlite": "sqlite", "mysql": "mysql", "sqlserver": "mssql"}[name])
	}
	std, err := sql.Open(name, dsn)
	if err != nil {
		return nil, fmt.Errorf("sqladapter: open: %w", err)
	}
	applyStdPool(std, d, dsn)
	if err := std.PingContext(ctx); err != nil {
		_ = std.Close()
		return nil, fmt.Errorf("sqladapter: ping: %w", err)
	}
	if !oteladapter.Disabled() && os.Getenv("NITRO_DB_QUIET") != "1" {
		_ = oteladapter.Ensure(os.Getenv("OTEL_SERVICE_NAME"))
	}
	gw := gateway.New(AdaptStdDB(std), withEnvQuiet(opts)...)
	return &DB{
		Driver:  d,
		std:     std,
		gw:      gw,
		dialect: dialectOf(d),
	}, nil
}

// errMissingDSN explains the common trap of a .env file that was never
// exported: OpenEnv reads the process environment only.
func errMissingDSN() error {
	if _, err := os.Stat(".env"); err == nil {
		return fmt.Errorf("sqladapter: missing DSN (set DATABASE_URL or DB_DSN; .env exists but is not in the process environment — call nitro.LoadDotEnv() before OpenEnv, or use nitrogen up)")
	}
	return fmt.Errorf("sqladapter: missing DSN (set DATABASE_URL or DB_DSN)")
}

func dialectOf(d Driver) dialect.Dialect {
	switch d {
	case DriverMySQL, DriverMariaDB:
		return dialect.MySQL()
	case DriverSQLServer:
		return dialect.MSSQL()
	case DriverSQLite:
		return dialect.SQLite()
	default:
		return dialect.Postgres()
	}
}

func ensureSQLiteDir(dsn string) error {
	path := dsn
	if strings.HasPrefix(path, "file:") {
		path = strings.TrimPrefix(path, "file:")
		if i := strings.IndexByte(path, '?'); i >= 0 {
			path = path[:i]
		}
	}
	if path == "" || path == ":memory:" || strings.Contains(path, "mode=memory") {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

func withEnvQuiet(opts []gateway.Option) []gateway.Option {
	if os.Getenv("NITRO_DB_QUIET") != "1" {
		return opts
	}
	out := make([]gateway.Option, 0, 1+len(opts))
	out = append(out, gateway.DisableObservability())
	return append(out, opts...)
}

// Close closes the pool.
func (d *DB) Close() {
	if d == nil {
		return
	}
	if d.pg != nil {
		d.pg.Close()
	}
	if d.std != nil {
		_ = d.std.Close()
	}
}

// Gateway returns the observability Gateway.
func (d *DB) Gateway() *gateway.Gateway {
	if d == nil {
		return nil
	}
	if d.pg != nil {
		return d.pg.Gateway
	}
	return d.gw
}

// Dialect is the BLinq SQL dialect.
func (d *DB) Dialect() dialect.Dialect {
	if d == nil {
		return dialect.Postgres()
	}
	return d.dialect
}

// Ping checks connectivity.
func (d *DB) Ping(ctx context.Context) error {
	if d == nil {
		return fmt.Errorf("sqladapter: db is nil")
	}
	if d.pg != nil {
		return d.pg.Pool.Ping(ctx)
	}
	if d.std == nil {
		return fmt.Errorf("sqladapter: db is nil")
	}
	return d.std.PingContext(ctx)
}

// StdDB returns *sql.DB for non-Postgres engines (nil on pgx path).
func (d *DB) StdDB() *sql.DB {
	if d == nil {
		return nil
	}
	return d.std
}

// Pgx returns the underlying pgx handle when Driver is postgresql.
func (d *DB) Pgx() *pgxadapter.DB {
	if d == nil {
		return nil
	}
	return d.pg
}

// StoreOf creates a typed BLinq Store (same API as pgx.StoreOf).
func StoreOf[T any](d *DB, table string, idCol ...string) *Store[T] {
	api := pgxadapter.NewDataAPIFromGatewayDialect(d.Gateway(), d.Dialect())
	return pgxadapter.NewStore[T](api, table, idCol...)
}
