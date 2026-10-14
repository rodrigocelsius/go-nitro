package query_test

import (
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func compileDX(q query.Query) (string, []any) {
	c, err := sql.Compile(q.AST(), dialect.Postgres())
	if err != nil {
		panic(err)
	}
	return c.SQL, c.Args
}

func TestTake_Skip(t *testing.T) {
	q := query.FromName("tracks").Take(6).Skip(12)
	got, _ := compileDX(q)
	want := `SELECT * FROM "tracks" LIMIT 6 OFFSET 12`
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestMonthBuckets(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	b := query.MonthBuckets(3, now)
	if len(b) != 3 {
		t.Fatalf("len=%d", len(b))
	}
	if b[0].Label != "2026-07" || b[2].Label != "2026-09" {
		t.Fatalf("%+v", b)
	}
	if !b[0].End.Equal(b[1].Start) {
		t.Fatalf("buckets must be contiguous")
	}
}

func TestCountGrouped_SQL(t *testing.T) {
	genreID := field.Col[*string]("tracks", "genre_id")
	q := query.From(field.Entity{Name: "tracks", SoftColumn: "deleted_at"}).
		GroupBy(genreID.Ref()).
		SelectItems(
			query.FieldSelect(genreID.Ref(), "key"),
			query.AggSelect(expr.Aggregate{Func: expr.AggCount}, "count"),
		).
		OrderBy(expr.Order{Field: expr.FieldRef{Column: "count"}, Dir: expr.OrderDesc}).
		Take(8)

	got, _ := compileDX(q)
	want := `SELECT "genre_id" AS "key", COUNT(*) AS "count" FROM "tracks" WHERE "deleted_at" IS NULL GROUP BY "genre_id" ORDER BY "count" DESC LIMIT 8`
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestInRange_SQL(t *testing.T) {
	created := field.Col[string]("tracks", "created_at")
	q := query.FromName("tracks").Where(created.InRange("a", "b"))
	got, args := compileDX(q)
	want := `SELECT * FROM "tracks" WHERE "created_at" >= $1 AND "created_at" < $2`
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 2 || args[0] != "a" || args[1] != "b" {
		t.Fatalf("args: %v", args)
	}
}

func TestEqValue_SQL(t *testing.T) {
	genreID := field.Col[*string]("tracks", "genre_id")
	q := query.FromName("tracks").Where(field.EqValue(genreID, "uuid-1"))
	got, args := compileDX(q)
	want := `SELECT * FROM "tracks" WHERE "genre_id" = $1`
	if got != want {
		t.Fatalf("SQL:\ngot:  %q\nwant: %q", got, want)
	}
	if len(args) != 1 {
		t.Fatalf("args: %v", args)
	}
	ptr, ok := args[0].(*string)
	if !ok || ptr == nil || *ptr != "uuid-1" {
		t.Fatalf("expected *string uuid-1, got %#v", args[0])
	}
}
