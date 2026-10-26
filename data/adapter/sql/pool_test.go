package sqladapter

import (
	"database/sql"
	"testing"
)

func TestApplyStdPool_EnvAndDefaults(t *testing.T) {
	t.Setenv("NITRO_DB_MAX_CONNS", "4")
	t.Setenv("NITRO_DB_MAX_IDLE_CONNS", "8")
	db, err := sql.Open("sqlite", "file:pooltest.db?mode=memory")
	if err != nil {
		t.Skip("sqlite driver not compiled in")
	}
	defer db.Close()

	applyStdPool(db, DriverMySQL, "user@tcp(localhost)/db")
	if got := db.Stats().MaxOpenConnections; got != 4 {
		t.Fatalf("MaxOpenConnections = %d, want 4", got)
	}

	t.Setenv("NITRO_DB_MAX_CONNS", "")
	db2, _ := sql.Open("sqlite", "file:pooltest2.db?mode=memory")
	defer db2.Close()
	applyStdPool(db2, DriverSQLite, "file:pooltest2.db?mode=memory")
	if got := db2.Stats().MaxOpenConnections; got != 0 {
		t.Fatalf("in-memory sqlite must keep defaults, got max=%d", got)
	}
}

func TestIsSQLiteMemory(t *testing.T) {
	if !isSQLiteMemory("file::memory:") || !isSQLiteMemory("file:x?mode=memory&cache=shared") || isSQLiteMemory("file:data/app.db") {
		t.Fatal("isSQLiteMemory mismatch")
	}
}

func TestApplyStdPool_DefaultBoundAndUnlimitedOptOut(t *testing.T) {
	t.Setenv("NITRO_DB_MAX_CONNS", "")
	db, err := sql.Open("sqlite", "file:pooltest3.db?mode=memory")
	if err != nil {
		t.Skip("sqlite driver not compiled in")
	}
	defer db.Close()
	applyStdPool(db, DriverMySQL, "user@tcp(localhost)/db")
	if got := db.Stats().MaxOpenConnections; got != defaultStdMaxOpenConns() {
		t.Fatalf("default MaxOpenConnections = %d, want %d", got, defaultStdMaxOpenConns())
	}
	t.Setenv("NITRO_DB_MAX_CONNS", "0")
	applyStdPool(db, DriverMySQL, "user@tcp(localhost)/db")
	if got := db.Stats().MaxOpenConnections; got != 0 {
		t.Fatalf("NITRO_DB_MAX_CONNS=0 must mean unlimited, got %d", got)
	}
}

func TestSQLiteDSNDefaults(t *testing.T) {
	cases := map[string]string{
		"file:data/app.db":                    "file:data/app.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate",
		"file:a.db?_pragma=busy_timeout(100)": "file:a.db?_pragma=busy_timeout(100)&_pragma=journal_mode(WAL)&_txlock=immediate",
		"file:a.db?_pragma=journal_mode(DELETE)&_pragma=busy_timeout(1)&_txlock=deferred": "file:a.db?_pragma=journal_mode(DELETE)&_pragma=busy_timeout(1)&_txlock=deferred",
		"file:x?mode=memory&cache=shared":                                                 "file:x?mode=memory&cache=shared",
	}
	for in, want := range cases {
		if got := sqliteDSNDefaults(in); got != want {
			t.Errorf("sqliteDSNDefaults(%q) = %q, want %q", in, got, want)
		}
	}
	t.Setenv("NITRO_SQLITE_PRAGMAS", "0")
	if got := sqliteDSNDefaults("file:a.db"); got != "file:a.db" {
		t.Fatalf("opt-out ignored: %q", got)
	}
}
