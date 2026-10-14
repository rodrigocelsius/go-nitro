// Package migrate applies versioned SQL migrations.
//
// Metadata placeholders (version INSERT/DELETE):
//   - BindPostgres (default): $1, $2 — compatible with pgx
//   - BindQMark: ? — SQLite / MySQL style
//
// File formats:
//   - Pairs: 000001_name.up.sql / 000001_name.down.sql (FSSource)
//   - Goose: 000001_name.sql with -- +goose Up / -- +goose Down (GooseFSSource)
//
// Generation from models: codegen/migration (emits goose .sql; convert or GooseFSSource).
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Migration is a versioned Up/Down unit.
type Migration struct {
	Version int64
	Name    string
	UpSQL   string
	DownSQL string
}

// Source provides the migration list.
type Source interface {
	Migrations() ([]Migration, error)
}

// Executor runs SQL (pgx: use data/adapter/pgx.AdaptMigrate).
type Executor interface {
	Exec(ctx context.Context, sql string, args []any) error
	Query(ctx context.Context, sql string, args []any) (Rows, error)
}

// TxStarter optionally enables transactional migrations. When Runner.Transactional
// is true and DB implements TxStarter, each migration's SQL + version record run
// atomically (supported on PostgreSQL / SQLite; MySQL DDL auto-commits).
type TxStarter interface {
	BeginTx(ctx context.Context) (TxExecutor, error)
}

// TxExecutor is a transaction-scoped Executor.
type TxExecutor interface {
	Executor
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Locker optionally serializes concurrent migrators (multi-instance deploys).
// The returned release function must be safe to call after ctx cancellation.
type Locker interface {
	AcquireLock(ctx context.Context, key string) (release func() error, err error)
}

// Rows iterates simple results.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

// Status describes a version's state.
type Status struct {
	Version   int64
	Name      string
	Applied   bool
	AppliedAt *time.Time
}

// BindStyle sets Runner placeholders (metadata), not migration SQL.
type BindStyle int

const (
	// BindPostgres uses $1, $2 (default — Nitro/pgx).
	BindPostgres BindStyle = iota
	// BindQMark uses ?.
	BindQMark
)

const schemaTable = "blinq_schema_migrations"

// Flavor selects CREATE TABLE SQL for the schema-version table.
type Flavor int

const (
	// FlavorPostgres uses TIMESTAMPTZ / NOW() (default).
	FlavorPostgres Flavor = iota
	// FlavorMySQL uses DATETIME(6) / CURRENT_TIMESTAMP (MySQL and MariaDB).
	FlavorMySQL
	// FlavorSQLite uses TEXT timestamps.
	FlavorSQLite
	// FlavorMSSQL uses DATETIMEOFFSET.
	FlavorMSSQL
)

// Runner applies/reverts migrations.
type Runner struct {
	DB     Executor
	Source Source
	Table  string    // default blinq_schema_migrations
	Bind   BindStyle // default BindPostgres
	Flavor Flavor    // default FlavorPostgres
	// Transactional wraps each migration (SQL + version record) in a
	// transaction when DB implements TxStarter. Opt-in: CREATE INDEX
	// CONCURRENTLY and friends cannot run inside one.
	Transactional bool
}

func (r *Runner) table() string {
	if r.Table != "" {
		return r.Table
	}
	return schemaTable
}

// quotedTable renders the metadata table identifier for the configured Flavor,
// so reserved words / unusual characters cannot break the generated SQL.
func (r *Runner) quotedTable() string {
	t := r.table()
	switch r.Flavor {
	case FlavorMySQL:
		return "`" + strings.ReplaceAll(t, "`", "``") + "`"
	case FlavorMSSQL:
		return "[" + strings.ReplaceAll(t, "]", "]]") + "]"
	default:
		return `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
	}
}

func (r *Runner) ph(n int) string {
	if r.Bind == BindQMark {
		return "?"
	}
	return "$" + strconv.Itoa(n)
}

// Up applies all pending migrations (ascending Version).
// When DB implements Locker, a migration lock is acquired for the whole run.
func (r *Runner) Up(ctx context.Context) error {
	if r.DB == nil {
		return fmt.Errorf("blinq/migrate: DB nil")
	}
	release, err := r.lock(ctx)
	if err != nil {
		return err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	if err := r.ensureTable(ctx); err != nil {
		return err
	}
	all, err := r.Source.Migrations()
	if err != nil {
		return err
	}
	applied, err := r.appliedSet(ctx)
	if err != nil {
		return err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Version < all[j].Version })
	for _, m := range all {
		if applied[m.Version] {
			continue
		}
		if strings.TrimSpace(m.UpSQL) == "" {
			return fmt.Errorf("blinq/migrate: version %d (%s) has empty UpSQL", m.Version, m.Name)
		}
		if err := r.applyUp(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// applyUp runs one migration's SQL and its version record — atomically when
// Transactional is set and the executor supports transactions.
func (r *Runner) applyUp(ctx context.Context, m Migration) error {
	starter, ok := r.DB.(TxStarter)
	if !r.Transactional || !ok {
		return r.runUp(ctx, r.DB, m)
	}
	tx, err := starter.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("blinq/migrate: begin tx up %d: %w", m.Version, err)
	}
	runErr := r.runUp(ctx, tx, m)
	if runErr == nil {
		if err := tx.Commit(ctx); err == nil {
			return nil
		} else {
			return fmt.Errorf("blinq/migrate: commit up %d: %w", m.Version, err)
		}
	}
	if rbErr := tx.Rollback(ctx); rbErr != nil {
		return fmt.Errorf("blinq/migrate: up %d %s: %v (rollback also failed: %v)", m.Version, m.Name, runErr, rbErr)
	}
	return runErr
}

func (r *Runner) runUp(ctx context.Context, ex Executor, m Migration) error {
	if err := ex.Exec(ctx, m.UpSQL, nil); err != nil {
		return fmt.Errorf("blinq/migrate: up %d %s: %w", m.Version, m.Name, err)
	}
	ins := fmt.Sprintf("INSERT INTO %s (version, applied_at) VALUES (%s, %s)", r.quotedTable(), r.ph(1), r.ph(2))
	var at any = time.Now().UTC()
	if r.Flavor == FlavorSQLite {
		// SQLite has no time type: store canonical text that reads back.
		at = at.(time.Time).Format(time.RFC3339Nano)
	}
	if err := ex.Exec(ctx, ins, []any{m.Version, at}); err != nil {
		return fmt.Errorf("blinq/migrate: record %d: %w", m.Version, err)
	}
	return nil
}

// lock acquires the cross-instance migration lock when supported.
func (r *Runner) lock(ctx context.Context) (func() error, error) {
	lm, ok := r.DB.(Locker)
	if !ok {
		return nil, nil
	}
	release, err := lm.AcquireLock(ctx, "blinq_migrate:"+r.table())
	if err != nil {
		return nil, fmt.Errorf("blinq/migrate: lock: %w", err)
	}
	return release, nil
}

// Down reverts the last `steps` applied migrations (descending order).
// When DB implements Locker, a migration lock is acquired for the whole run.
func (r *Runner) Down(ctx context.Context, steps int) error {
	if steps <= 0 {
		return nil
	}
	if r.DB == nil {
		return fmt.Errorf("blinq/migrate: DB nil")
	}
	release, err := r.lock(ctx)
	if err != nil {
		return err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	if err := r.ensureTable(ctx); err != nil {
		return err
	}
	all, err := r.Source.Migrations()
	if err != nil {
		return err
	}
	byVer := map[int64]Migration{}
	for _, m := range all {
		byVer[m.Version] = m
	}
	appliedList, err := r.appliedVersions(ctx)
	if err != nil {
		return err
	}
	sort.Slice(appliedList, func(i, j int) bool { return appliedList[i] > appliedList[j] })
	n := steps
	if n > len(appliedList) {
		n = len(appliedList)
	}
	for i := 0; i < n; i++ {
		ver := appliedList[i]
		m, ok := byVer[ver]
		if !ok {
			return fmt.Errorf("blinq/migrate: applied version %d not in source", ver)
		}
		if strings.TrimSpace(m.DownSQL) == "" {
			return fmt.Errorf("blinq/migrate: version %d (%s) has empty DownSQL", m.Version, m.Name)
		}
		run := func(ex Executor) error {
			if err := ex.Exec(ctx, m.DownSQL, nil); err != nil {
				return fmt.Errorf("blinq/migrate: down %d %s: %w", ver, m.Name, err)
			}
			del := fmt.Sprintf("DELETE FROM %s WHERE version = %s", r.quotedTable(), r.ph(1))
			if err := ex.Exec(ctx, del, []any{ver}); err != nil {
				return fmt.Errorf("blinq/migrate: unrecord %d: %w", ver, err)
			}
			return nil
		}
		if starter, ok := r.DB.(TxStarter); ok && r.Transactional {
			tx, txErr := starter.BeginTx(ctx)
			if txErr != nil {
				return fmt.Errorf("blinq/migrate: begin tx down %d: %w", ver, txErr)
			}
			if runErr := run(tx); runErr != nil {
				if rbErr := tx.Rollback(ctx); rbErr != nil {
					return fmt.Errorf("blinq/migrate: down %d rollback failed after error: %v (rb: %v)", ver, runErr, rbErr)
				}
				return runErr
			}
			if cErr := tx.Commit(ctx); cErr != nil {
				return fmt.Errorf("blinq/migrate: commit down %d: %w", ver, cErr)
			}
			continue
		}
		if err := run(r.DB); err != nil {
			return err
		}
	}
	return nil
}

// Status lists Source migrations with the Applied flag.
func (r *Runner) Status(ctx context.Context) ([]Status, error) {
	if err := r.ensureTable(ctx); err != nil {
		return nil, err
	}
	all, err := r.Source.Migrations()
	if err != nil {
		return nil, err
	}
	applied, times, err := r.appliedMap(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Version < all[j].Version })
	out := make([]Status, 0, len(all))
	for _, m := range all {
		st := Status{Version: m.Version, Name: m.Name, Applied: applied[m.Version]}
		if t, ok := times[m.Version]; ok {
			tt := t
			st.AppliedAt = &tt
		}
		out = append(out, st)
	}
	return out, nil
}

func (r *Runner) ensureTable(ctx context.Context) error {
	tbl := r.quotedTable()
	raw := strings.ReplaceAll(r.table(), "'", "''")
	var sql string
	switch r.Flavor {
	case FlavorMySQL:
		sql = fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s (version BIGINT PRIMARY KEY, applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6))`,
			tbl,
		)
	case FlavorSQLite:
		sql = fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (datetime('now')))`,
			tbl,
		)
	case FlavorMSSQL:
		sql = fmt.Sprintf(
			`IF OBJECT_ID(N'%s', N'U') IS NULL CREATE TABLE %s (version BIGINT PRIMARY KEY, applied_at DATETIMEOFFSET NOT NULL DEFAULT SYSUTCDATETIME())`,
			raw, tbl,
		)
	default:
		sql = fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s (version BIGINT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
			tbl,
		)
	}
	return r.DB.Exec(ctx, sql, nil)
}

func (r *Runner) appliedSet(ctx context.Context) (map[int64]bool, error) {
	m, _, err := r.appliedMap(ctx)
	return m, err
}

func (r *Runner) appliedVersions(ctx context.Context) ([]int64, error) {
	_, times, err := r.appliedMap(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(times))
	for v := range times {
		out = append(out, v)
	}
	return out, nil
}

func (r *Runner) appliedMap(ctx context.Context) (map[int64]bool, map[int64]time.Time, error) {
	sql := fmt.Sprintf("SELECT version, applied_at FROM %s", r.quotedTable())
	rows, err := r.DB.Query(ctx, sql, nil)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	applied := map[int64]bool{}
	times := map[int64]time.Time{}
	for rows.Next() {
		var ver int64
		var raw any
		if err := rows.Scan(&ver, &raw); err != nil {
			return nil, nil, err
		}
		applied[ver] = true
		times[ver] = parseAppliedAt(raw)
	}
	return applied, times, rows.Err()
}

// appliedAtLayouts covers what drivers hand back for applied_at when it is not
// a time.Time: RFC 3339, SQLite datetime('now'), MySQL without parseTime, and
// Go's time.Time.String() (written by older SQLite runners).
var appliedAtLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
}

// parseAppliedAt converts a scanned applied_at value to time.Time. An
// unparseable value yields the zero time: the timestamp is informational and
// must never block migrations.
func parseAppliedAt(raw any) time.Time {
	var s string
	switch v := raw.(type) {
	case time.Time:
		return v
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return time.Time{}
	}
	s = strings.TrimSpace(s)
	// time.Time.String() may append a monotonic clock reading ("m=+0.01").
	if i := strings.Index(s, " m="); i > 0 {
		s = s[:i]
	}
	for _, layout := range appliedAtLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// FSSource reads NNNNNN_name.up.sql / .down.sql pairs from an fs.FS.
type FSSource struct {
	FS fs.FS
}

func (s FSSource) Migrations() ([]Migration, error) {
	entries, err := fs.ReadDir(s.FS, ".")
	if err != nil {
		return nil, err
	}
	type pair struct {
		up, down string
		name     string
		ver      int64
	}
	by := map[int64]*pair{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ver, migName, kind, ok := parseMigrationFile(name)
		if !ok {
			continue
		}
		p := by[ver]
		if p == nil {
			p = &pair{ver: ver, name: migName}
			by[ver] = p
		}
		data, err := fs.ReadFile(s.FS, name)
		if err != nil {
			return nil, err
		}
		switch kind {
		case "up":
			p.up = string(data)
		case "down":
			p.down = string(data)
		}
		if p.name == "" {
			p.name = migName
		}
	}
	out := make([]Migration, 0, len(by))
	for _, p := range by {
		out = append(out, Migration{
			Version: p.ver,
			Name:    p.name,
			UpSQL:   p.up,
			DownSQL: p.down,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// GooseFSSource reads NNNNNN_name.sql with -- +goose Up / -- +goose Down markers.
type GooseFSSource struct {
	FS fs.FS
}

func (s GooseFSSource) Migrations() ([]Migration, error) {
	entries, err := fs.ReadDir(s.FS, ".")
	if err != nil {
		return nil, err
	}
	var out []Migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		// skip pair style
		if strings.Contains(e.Name(), ".up.sql") || strings.Contains(e.Name(), ".down.sql") {
			continue
		}
		ver, name, ok := parseGooseFilename(e.Name())
		if !ok {
			continue
		}
		data, err := fs.ReadFile(s.FS, e.Name())
		if err != nil {
			return nil, err
		}
		up, down := splitGoose(string(data))
		out = append(out, Migration{Version: ver, Name: name, UpSQL: up, DownSQL: down})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func parseGooseFilename(filename string) (version int64, name string, ok bool) {
	if !strings.HasSuffix(filename, ".sql") {
		return 0, "", false
	}
	base := strings.TrimSuffix(filename, ".sql")
	i := strings.IndexByte(base, '_')
	if i <= 0 {
		return 0, "", false
	}
	ver, err := strconv.ParseInt(base[:i], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return ver, base[i+1:], true
}

func splitGoose(content string) (up, down string) {
	const (
		upMark   = "-- +goose Up"
		downMark = "-- +goose Down"
	)
	u := strings.Index(content, upMark)
	d := strings.Index(content, downMark)
	if u < 0 && d < 0 {
		return strings.TrimSpace(content), ""
	}
	if u >= 0 && d > u {
		up = strings.TrimSpace(content[u+len(upMark) : d])
		down = strings.TrimSpace(content[d+len(downMark):])
		return up, down
	}
	if u >= 0 {
		return strings.TrimSpace(content[u+len(upMark):]), ""
	}
	if d >= 0 {
		return "", strings.TrimSpace(content[d+len(downMark):])
	}
	return strings.TrimSpace(content), ""
}

func parseMigrationFile(filename string) (version int64, name, kind string, ok bool) {
	if !strings.HasSuffix(filename, ".sql") {
		return 0, "", "", false
	}
	base := strings.TrimSuffix(filename, ".sql")
	var suffix string
	switch {
	case strings.HasSuffix(base, ".up"):
		kind = "up"
		suffix = ".up"
	case strings.HasSuffix(base, ".down"):
		kind = "down"
		suffix = ".down"
	default:
		return 0, "", "", false
	}
	base = strings.TrimSuffix(base, suffix)
	i := strings.IndexByte(base, '_')
	if i <= 0 {
		return 0, "", "", false
	}
	ver, err := strconv.ParseInt(base[:i], 10, 64)
	if err != nil {
		return 0, "", "", false
	}
	return ver, base[i+1:], kind, true
}

// MemorySource is a fixed list (tests / manual wiring).
type MemorySource []Migration

func (m MemorySource) Migrations() ([]Migration, error) {
	out := append([]Migration(nil), m...)
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}
