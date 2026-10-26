package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

type captureDB struct{ sql string }

func (c *captureDB) QueryMaps(_ context.Context, sql string, _ []any) ([]map[string]any, error) {
	c.sql = sql
	return nil, nil
}
func (c *captureDB) Exec(_ context.Context, _ string, sql string, _ []any) (int64, error) {
	c.sql = sql
	return 0, nil
}
func (c *captureDB) QueryRowScan(_ context.Context, sql string, _ []any, _ ...any) error {
	c.sql = sql
	return nil
}

func TestNew_UsesPostgresDialect(t *testing.T) {
	db := &captureDB{}
	p := New(db)
	q := query.From(field.Entity{Name: "users"}).Where(field.Col[string]("users", "name").Eq("ada"))
	if _, err := p.Execute(context.Background(), q.AST()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(db.sql, "$1") || !strings.Contains(db.sql, `"users"`) {
		t.Fatalf("expected Postgres placeholders/quoting, got %s", db.sql)
	}
}
