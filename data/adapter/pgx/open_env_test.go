package pgx_test

import (
	"context"
	"strings"
	"testing"

	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
)

func TestOpen_EmptyDSN_Error(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	_, err := pgxadapter.Open(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "missing DSN") {
		t.Fatalf("got %v", err)
	}
	_, err = pgxadapter.OpenEnv(context.Background())
	if err == nil || !strings.Contains(err.Error(), "missing DSN") {
		t.Fatalf("OpenEnv: %v", err)
	}
}
