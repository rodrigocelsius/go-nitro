package sqladapter

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/migrate"
)

const defaultMigrationsDir = "migrations"

// MigrateUp applies ./migrations (goose).
func (d *DB) MigrateUp(ctx context.Context) error {
	return d.MigrateDir(ctx, defaultMigrationsDir)
}

// MigrateDir applies goose files from a directory.
func (d *DB) MigrateDir(ctx context.Context, dir string) error {
	if dir == "" {
		dir = defaultMigrationsDir
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("sqladapter: migrate dir %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("sqladapter: migrate dir %s: not a directory", dir)
	}
	return d.MigrateFS(ctx, os.DirFS(dir))
}

// MigrateFS applies goose files (NNNNNN_name.sql).
func (d *DB) MigrateFS(ctx context.Context, fsys fs.FS) error {
	if d.pg != nil {
		return d.pg.MigrateFS(ctx, fsys)
	}
	if sub, ok := migrationsSub(fsys); ok {
		fsys = sub
	}
	r := &migrate.Runner{
		DB:     adaptMigrate(d.std),
		Source: migrate.GooseFSSource{FS: fsys},
		Bind:   migrate.BindQMark,
		Flavor: migrateFlavor(d.Driver),
	}
	return r.Up(ctx)
}

func migrateFlavor(d Driver) migrate.Flavor {
	switch d {
	case DriverMySQL, DriverMariaDB:
		return migrate.FlavorMySQL
	case DriverSQLite:
		return migrate.FlavorSQLite
	case DriverSQLServer:
		return migrate.FlavorMSSQL
	default:
		return migrate.FlavorPostgres
	}
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
