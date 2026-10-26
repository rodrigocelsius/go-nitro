package pgx

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStripPgBouncerParam(t *testing.T) {
	cases := []struct {
		in, out string
		on      bool
	}{
		{"postgres://u:p@h:6432/db?pgbouncer=true&sslmode=disable", "postgres://u:p@h:6432/db?sslmode=disable", true},
		{"postgresql://u@h/db?pgbouncer=false", "postgresql://u@h/db", false},
		{"postgres://u@h/db?sslmode=disable", "postgres://u@h/db?sslmode=disable", false},
		{"host=h port=6432 pgbouncer=1 dbname=db", "host=h port=6432 dbname=db", true},
		{"host=h password='a b' dbname=db", "host=h password='a b' dbname=db", false},
	}
	for _, c := range cases {
		got, on := stripPgBouncerParam(c.in)
		if got != c.out || on != c.on {
			t.Errorf("stripPgBouncerParam(%q) = %q,%v want %q,%v", c.in, got, on, c.out, c.on)
		}
	}
}

func TestConfigurePgBouncer(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://u@localhost:6432/db")
	if err != nil {
		t.Fatal(err)
	}
	configurePgBouncer(cfg, "postgres://u@localhost:6432/db")
	if cfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeSimpleProtocol {
		t.Fatalf("mode = %v, want simple protocol", cfg.ConnConfig.DefaultQueryExecMode)
	}
	if cfg.ConnConfig.StatementCacheCapacity != 0 {
		t.Fatal("statement cache must be off")
	}

	// An explicit mode in the DSN wins.
	dsn := "postgres://u@localhost:6432/db?default_query_exec_mode=exec"
	cfg, _ = pgxpool.ParseConfig(dsn)
	configurePgBouncer(cfg, dsn)
	if cfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeExec {
		t.Fatalf("explicit mode overridden: %v", cfg.ConnConfig.DefaultQueryExecMode)
	}
}

func TestApplyPoolEnv(t *testing.T) {
	t.Setenv("NITRO_DB_MAX_CONNS", "42")
	t.Setenv("NITRO_DB_MIN_CONNS", "3")
	t.Setenv("NITRO_DB_CONN_MAX_LIFETIME", "10m")
	t.Setenv("NITRO_DB_CONN_MAX_IDLE_TIME", "bad")
	cfg, _ := pgxpool.ParseConfig("postgres://u@localhost/db")
	idle := cfg.MaxConnIdleTime
	applyPoolEnv(cfg)
	if cfg.MaxConns != 42 || cfg.MinConns != 3 || cfg.MaxConnLifetime != 10*time.Minute {
		t.Fatalf("max=%d min=%d life=%v", cfg.MaxConns, cfg.MinConns, cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdleTime != idle {
		t.Fatal("invalid duration must be ignored")
	}
}

func TestDirectDSN(t *testing.T) {
	t.Setenv("DATABASE_DIRECT_URL", "")
	t.Setenv("NITRO_DB_DIRECT_URL", "postgres://direct")
	if DirectDSN() != "postgres://direct" {
		t.Fatal(DirectDSN())
	}
}

func TestDetectPgBouncerPort(t *testing.T) {
	t.Setenv("NITRO_DB_PGBOUNCER", "")
	on, _ := pgxpool.ParseConfig("postgres://u@h:6432/db")
	off, _ := pgxpool.ParseConfig("postgres://u@h:5432/db")
	if !detectPgBouncerPort(on) || detectPgBouncerPort(off) {
		t.Fatal("port 6432 must be detected, 5432 not")
	}
	t.Setenv("NITRO_DB_PGBOUNCER", "0")
	if detectPgBouncerPort(on) {
		t.Fatal("NITRO_DB_PGBOUNCER=0 must opt out")
	}
}
