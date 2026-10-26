package sqladapter

import (
	"database/sql"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Pool defaults for database/sql engines (MySQL, MariaDB, SQL Server).
// database/sql keeps only 2 idle connections by default, so a burst of
// concurrent requests opens and closes connections on every request; and
// connections without a lifetime outlive proxies (ProxySQL, Azure SQL
// gateway, load balancers) that silently drop idle TCP sessions.
const (
	defaultStdMaxIdleConns    = 10
	defaultStdConnMaxLifetime = 30 * time.Minute
	defaultStdConnMaxIdleTime = 5 * time.Minute
)

// defaultStdMaxOpenConns bounds open connections when NITRO_DB_MAX_CONNS is
// unset. database/sql's own default is unlimited: under a burst every waiting
// request opens a connection until the server refuses ("too many
// connections", MySQL max_connections = 151). With a bound, extra requests
// queue inside the process for a free connection instead — the in-process
// pooling every engine gets without an external pooler.
// defaultSQLiteMaxOpenConns is the pool bound for SQLite files.
const defaultSQLiteMaxOpenConns = 4

func defaultStdMaxOpenConns() int {
	return max(10, 4*runtime.NumCPU())
}

// applyStdPool sizes the *sql.DB pool from the environment (same variables
// as the pgx pool):
//
//	NITRO_DB_MAX_CONNS           max open connections (default max(10, 4×CPU), SQLite file 4; 0 or negative = unlimited)
//	NITRO_DB_MAX_IDLE_CONNS      idle connections kept (default 10, capped at max)
//	NITRO_DB_CONN_MAX_LIFETIME   e.g. 30m (default 30m)
//	NITRO_DB_CONN_MAX_IDLE_TIME  e.g. 5m  (default 5m)
//
// In-memory SQLite keeps database/sql defaults: closing its last connection
// would drop the database.
func applyStdPool(db *sql.DB, d Driver, dsn string) {
	if d == DriverSQLite && isSQLiteMemory(dsn) {
		if n, ok := envInt("NITRO_DB_MAX_CONNS"); ok && n > 0 {
			db.SetMaxOpenConns(n)
		}
		return
	}
	maxOpen := defaultStdMaxOpenConns()
	if d == DriverSQLite {
		// One writer at a time: a small pool keeps WAL readers concurrent
		// while writers queue in Go instead of sleeping on busy_timeout
		// (64 concurrent inserts: ~0.5s with 4 connections, ~3s with 32).
		maxOpen = defaultSQLiteMaxOpenConns
	}
	if n, ok := envInt("NITRO_DB_MAX_CONNS"); ok {
		maxOpen = n
	}
	db.SetMaxOpenConns(max(maxOpen, 0)) // 0 = unlimited (explicit opt-out)
	idle := defaultStdMaxIdleConns
	if n, ok := envInt("NITRO_DB_MAX_IDLE_CONNS"); ok && n >= 0 {
		idle = n
	}
	if maxOpen > 0 && idle > maxOpen {
		idle = maxOpen
	}
	db.SetMaxIdleConns(idle)
	life := defaultStdConnMaxLifetime
	if v, ok := envDuration("NITRO_DB_CONN_MAX_LIFETIME"); ok {
		life = v
	}
	db.SetConnMaxLifetime(life)
	idleTime := defaultStdConnMaxIdleTime
	if v, ok := envDuration("NITRO_DB_CONN_MAX_IDLE_TIME"); ok {
		idleTime = v
	}
	db.SetConnMaxIdleTime(idleTime)
}

func isSQLiteMemory(dsn string) bool {
	return strings.Contains(dsn, ":memory:") || strings.Contains(dsn, "mode=memory")
}

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

// sqliteDSNDefaults adds, unless the DSN already sets them,
// _pragma=busy_timeout(5000) — a writer waits up to 5s for the lock instead
// of failing at once with "database is locked" (SQLITE_BUSY) — and
// _pragma=journal_mode(WAL) — readers no longer block the writer. In-memory
// databases are left untouched. Also _txlock=immediate (see below).
func sqliteDSNDefaults(dsn string) string {
	if isSQLiteMemory(dsn) || os.Getenv("NITRO_SQLITE_PRAGMAS") == "0" {
		return dsn
	}
	base, query, _ := strings.Cut(dsn, "?")
	vals, err := url.ParseQuery(query)
	if err != nil {
		return dsn
	}
	has := func(name string) bool {
		for _, p := range vals["_pragma"] {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(p)), name) {
				return true
			}
		}
		return false
	}
	add := make([]string, 0, 2)
	if !has("busy_timeout") {
		add = append(add, "_pragma=busy_timeout(5000)")
	}
	if !has("journal_mode") {
		add = append(add, "_pragma=journal_mode(WAL)")
	}
	if _, ok := vals["_txlock"]; !ok {
		// BEGIN IMMEDIATE: a transaction takes the write lock up front, so two
		// writers queue on busy_timeout instead of deadlocking on lock upgrade.
		add = append(add, "_txlock=immediate")
	}
	if len(add) == 0 {
		return dsn
	}
	if query == "" {
		return base + "?" + strings.Join(add, "&")
	}
	return dsn + "&" + strings.Join(add, "&")
}
