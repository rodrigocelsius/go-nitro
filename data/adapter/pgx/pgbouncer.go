package pgx

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// stripPgBouncerParam removes the `pgbouncer=true|1` flag from a URL DSN
// (pgx would forward unknown params to the server as runtime parameters and
// the connection would fail) and reports whether it was set.
func stripPgBouncerParam(dsn string) (string, bool) {
	if !strings.Contains(dsn, "pgbouncer") {
		return dsn, false
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return dsn, false
		}
		q := u.Query()
		v, ok := q["pgbouncer"]
		if !ok {
			return dsn, false
		}
		q.Del("pgbouncer")
		u.RawQuery = q.Encode()
		return u.String(), len(v) > 0 && isTruthy(v[0])
	}
	// keyword/value DSN: "host=… pgbouncer=true"
	fields := strings.Fields(dsn)
	out := make([]string, 0, len(fields))
	on, found := false, false
	for _, f := range fields {
		if k, v, ok := strings.Cut(f, "="); ok && k == "pgbouncer" {
			on, found = isTruthy(v), true
			continue
		}
		out = append(out, f)
	}
	if !found {
		return dsn, false
	}
	return strings.Join(out, " "), on
}

// pgBouncerPort is PgBouncer's default listen port.
const pgBouncerPort = 6432

// detectPgBouncerPort is the fallback when nothing enabled PgBouncer mode
// explicitly: a DSN on port 6432 almost always points at PgBouncer, and the
// simple protocol is safe on plain Postgres too. NITRO_DB_PGBOUNCER=0 opts out.
func detectPgBouncerPort(cfg *pgxpool.Config) bool {
	if v := strings.TrimSpace(os.Getenv("NITRO_DB_PGBOUNCER")); v != "" && !isTruthy(v) {
		return false
	}
	return cfg.ConnConfig.Port == pgBouncerPort
}

// configurePgBouncer switches to the simple query protocol unless the DSN
// chose a default_query_exec_mode explicitly. Named prepared statements
// (pgx's default cache_statement mode) break under PgBouncer transaction
// pooling: the statement is prepared on one server connection and executed
// on another ("prepared statement does not exist").
func configurePgBouncer(cfg *pgxpool.Config, dsn string) {
	if !strings.Contains(dsn, "default_query_exec_mode") {
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}
	cfg.ConnConfig.StatementCacheCapacity = 0
	cfg.ConnConfig.DescriptionCacheCapacity = 0
}

// applyPoolEnv applies NITRO_DB_MAX_CONNS / NITRO_DB_MIN_CONNS /
// NITRO_DB_CONN_MAX_LIFETIME / NITRO_DB_CONN_MAX_IDLE_TIME (PoolConfig wins).
func applyPoolEnv(cfg *pgxpool.Config) {
	if n, ok := envInt("NITRO_DB_MAX_CONNS"); ok && n > 0 {
		cfg.MaxConns = int32(n)
	}
	if n, ok := envInt("NITRO_DB_MIN_CONNS"); ok && n >= 0 {
		cfg.MinConns = int32(n)
	}
	if d, ok := envDuration("NITRO_DB_CONN_MAX_LIFETIME"); ok {
		cfg.MaxConnLifetime = d
	}
	if d, ok := envDuration("NITRO_DB_CONN_MAX_IDLE_TIME"); ok {
		cfg.MaxConnIdleTime = d
	}
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func envBool(key string) bool { return isTruthy(os.Getenv(key)) }

func envInt(key string) (int, bool) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	return n, err == nil
}

func envDuration(key string) (time.Duration, bool) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return 0, false
	}
	d, err := time.ParseDuration(v)
	return d, err == nil && d > 0
}

// DirectDSN returns DATABASE_DIRECT_URL (then NITRO_DB_DIRECT_URL): a DSN that
// bypasses PgBouncer, used for migrations (session advisory lock, DDL).
func DirectDSN() string {
	for _, k := range []string{"DATABASE_DIRECT_URL", "NITRO_DB_DIRECT_URL"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
