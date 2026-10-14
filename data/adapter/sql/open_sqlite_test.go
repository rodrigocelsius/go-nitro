package sqladapter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSQLiteMemory(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, "sqlite", "file:memdb1?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if db.Driver != DriverSQLite {
		t.Fatalf("driver %s", db.Driver)
	}
	if db.Dialect().ReturningSupported() {
		t.Fatal("sqlite has no RETURNING in Nitro dialect")
	}
	if err := db.Ping(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestOpenTest(t *testing.T) {
	ctx := context.Background()
	db, err := OpenTest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	db2, err := OpenTestFile(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
}

func TestOpenTestMigrateEmpty(t *testing.T) {
	ctx := context.Background()
	db, err := OpenTestMigrate(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
}

// Restarting an app re-runs MigrateUp against an already-migrated SQLite file.
// It used to fail with `Scan error on column "applied_at"`, because the
// timestamp was written as time.Time.String() text that could not be read back.
func TestSQLiteMigrateUpTwiceAcrossReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	migDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}
	goose := "-- +goose Up\nCREATE TABLE items (id TEXT PRIMARY KEY);\n-- +goose Down\nDROP TABLE items;\n"
	if err := os.WriteFile(filepath.Join(migDir, "20260101000000_create_items.sql"), []byte(goose), 0o644); err != nil {
		t.Fatal(err)
	}
	dsn := "file:" + filepath.Join(dir, "app.db")
	for boot := 1; boot <= 2; boot++ {
		db, err := Open(ctx, "sqlite", dsn)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.MigrateDir(ctx, migDir); err != nil {
			db.Close()
			t.Fatalf("boot %d: MigrateUp: %v", boot, err)
		}
		db.Close()
	}
}

// Databases written by the old runner hold time.Time.String() text in
// applied_at; they must keep migrating.
func TestSQLiteMigrateUpReadsLegacyAppliedAt(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	migDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}
	goose := "-- +goose Up\nCREATE TABLE items (id TEXT PRIMARY KEY);\n-- +goose Down\nDROP TABLE items;\n"
	if err := os.WriteFile(filepath.Join(migDir, "20260101000000_create_items.sql"), []byte(goose), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, "sqlite", "file:"+filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.MigrateDir(ctx, migDir); err != nil {
		t.Fatal(err)
	}
	legacy := "2026-10-14 17:47:19.226088917 +0000 UTC"
	if _, err := db.StdDB().ExecContext(ctx, `UPDATE blinq_schema_migrations SET applied_at = ?`, legacy); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateDir(ctx, migDir); err != nil {
		t.Fatalf("MigrateUp with legacy applied_at: %v", err)
	}
}
