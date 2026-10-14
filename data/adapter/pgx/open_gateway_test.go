package pgx_test

import (
	"os"
	"testing"

	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
)

func TestDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	if got := pgxadapter.DSN("fallback"); got != "fallback" {
		t.Fatalf("%q", got)
	}
	t.Setenv("DATABASE_URL", "postgres://x")
	if got := pgxadapter.DSN("fallback"); got != "postgres://x" {
		t.Fatalf("%q", got)
	}
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "postgres://y")
	if got := pgxadapter.EnvDSN(); got != "postgres://y" {
		t.Fatalf("EnvDSN %q", got)
	}
}

func TestEnvQuietDocumented(t *testing.T) {
	if os.Getenv("NITRO_DB_QUIET") == "1" {
		t.Log("quiet mode on")
	}
}
