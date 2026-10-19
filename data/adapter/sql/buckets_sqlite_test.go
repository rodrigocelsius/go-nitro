package sqladapter

import (
	"context"
	"testing"
	"time"

	blinqfield "github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

type sqliteBucketRow struct {
	ID        string    `db:"id"`
	Tenant    string    `db:"tenant"`
	CreatedAt time.Time `db:"created_at"`
}

// Positional "?" placeholders: CASE args precede WHERE args in the SQL text.
func TestCountBucketsWhere_SQLitePositionalPlaceholders(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, "sqlite", "file:bucketsdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	std := db.StdDB()
	if _, err := std.ExecContext(ctx, `CREATE TABLE b_rows (id TEXT PRIMARY KEY, tenant TEXT, created_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	rows := []struct {
		id, tenant string
		at         time.Time
	}{
		{"1", "a", now}, {"2", "a", now.AddDate(0, 0, -1)}, {"3", "b", now}, {"4", "a", now.AddDate(0, 0, -9)},
	}
	for _, r := range rows {
		if _, err := std.ExecContext(ctx, `INSERT INTO b_rows VALUES (?, ?, ?)`, r.id, r.tenant, r.at.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	store := StoreOf[sqliteBucketRow](db, "b_rows")
	got, err := store.CountBucketsWhere(ctx, blinqfield.Col[time.Time]("b_rows", "created_at").Ref(),
		query.DayBuckets(2, now), blinqfield.Col[string]("b_rows", "tenant").Eq("a"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Count != 1 || got[1].Count != 1 {
		t.Fatalf("got %+v", got)
	}
}
