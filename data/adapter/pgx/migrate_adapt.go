package pgx

import (
	"context"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/migrate"
)

const defaultMigrationsDir = "migrations"

// AdaptMigrate adapts Pool to migrate.Executor ($N placeholders — BindPostgres).
// The returned executor also implements migrate.TxStarter (transactional
// migrations) and migrate.Locker (pg_advisory_lock pinned to one connection).
func AdaptMigrate(pool *Pool) migrate.Executor {
	return &migrateExec{pool: pool}
}

// Migrate applies pending migrations via blinq/migrate.Runner.
// Transactional per-migration + advisory lock are enabled (Postgres supports both).
func (d *DB) Migrate(ctx context.Context, source migrate.Source) error {
	if d == nil || d.Pool == nil {
		return fmt.Errorf("pgx: db is nil")
	}
	r := &migrate.Runner{
		DB:            AdaptMigrate(d.Pool),
		Source:        source,
		Transactional: true,
	}
	return r.Up(ctx)
}

// MigrateUp applies ./migrations (goose). Shortcut for hello-world / local apps.
//
//	_ = db.MigrateUp(ctx)
func (d *DB) MigrateUp(ctx context.Context) error {
	return d.MigrateDir(ctx, defaultMigrationsDir)
}

// MigrateDir applies migrations from a directory on disk.
func (d *DB) MigrateDir(ctx context.Context, dir string) error {
	if dir == "" {
		dir = defaultMigrationsDir
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("pgx: migrate dir %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("pgx: migrate dir %s: not a directory", dir)
	}
	return d.MigrateFS(ctx, os.DirFS(dir))
}

// MigrateFS applies goose files (NNNNNN_name.sql).
// If the FS has a "migrations/" folder, uses it; otherwise the root.
func (d *DB) MigrateFS(ctx context.Context, fsys fs.FS) error {
	if sub, ok := migrationsSub(fsys); ok {
		fsys = sub
	}
	return d.Migrate(ctx, migrate.GooseFSSource{FS: fsys})
}

func migrationsSub(fsys fs.FS) (fs.FS, bool) {
	if _, err := fs.Stat(fsys, defaultMigrationsDir); err != nil {
		return nil, false
	}
	sub, err := fs.Sub(fsys, defaultMigrationsDir)
	if err != nil {
		return nil, false
	}
	return sub, true
}

type migrateExec struct {
	pool *Pool
}

func (e *migrateExec) Exec(ctx context.Context, sql string, args []any) error {
	if e.pool == nil || e.pool.DB() == nil {
		return fmt.Errorf("pgx: migrate pool nil")
	}
	_, err := e.pool.DB().Exec(ctx, sql, args...)
	return err
}

func (e *migrateExec) Query(ctx context.Context, sql string, args []any) (migrate.Rows, error) {
	if e.pool == nil || e.pool.DB() == nil {
		return nil, fmt.Errorf("pgx: migrate pool nil")
	}
	rows, err := e.pool.DB().Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &migrateRows{rows: rows}, nil
}

// BeginTx implements migrate.TxStarter.
func (e *migrateExec) BeginTx(ctx context.Context) (migrate.TxExecutor, error) {
	if e.pool == nil || e.pool.DB() == nil {
		return nil, fmt.Errorf("pgx: migrate pool nil")
	}
	tx, err := e.pool.DB().Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &migrateTx{tx: tx}, nil
}

// AcquireLock implements migrate.Locker via pg_advisory_lock pinned to a
// single pooled connection — session-scoped locks require it.
func (e *migrateExec) AcquireLock(ctx context.Context, key string) (func() error, error) {
	if e.pool == nil || e.pool.DB() == nil {
		return nil, fmt.Errorf("pgx: migrate pool nil")
	}
	conn, err := e.pool.DB().Acquire(ctx)
	if err != nil {
		return nil, err
	}
	lockID := advisoryLockID(key)
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		conn.Release()
		return nil, fmt.Errorf("pgx: advisory lock: %w", err)
	}
	released := false
	return func() error {
		if released {
			return nil
		}
		released = true
		defer conn.Release()
		// Unlock uses a fresh context: the parent may already be cancelled
		// during shutdown, and the lock is bound to the session anyway.
		_, err := conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockID)
		if err != nil {
			// Connection close releases the lock server-side; not fatal here.
			return nil
		}
		return nil
	}, nil
}

// advisoryLockKey hashes the key into the signed-bigint space required by
// pg_advisory_lock(classid int8).
func advisoryLockID(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	v := int64(h.Sum64())
	if v < 0 {
		v = -v
	}
	return v
}

type migrateTx struct {
	tx pgx.Tx
}

func (t *migrateTx) Exec(ctx context.Context, sql string, args []any) error {
	_, err := t.tx.Exec(ctx, sql, args...)
	return err
}

func (t *migrateTx) Query(ctx context.Context, sql string, args []any) (migrate.Rows, error) {
	rows, err := t.tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &migrateRows{rows: rows}, nil
}

func (t *migrateTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *migrateTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

type migrateRows struct {
	rows pgx.Rows
}

func (r *migrateRows) Next() bool             { return r.rows.Next() }
func (r *migrateRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *migrateRows) Close()                 { r.rows.Close() }
func (r *migrateRows) Err() error             { return r.rows.Err() }
