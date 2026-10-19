//go:build integration

package sqladapter_test

import (
	"context"
	"os"
	"testing"
	"time"

	blinqfield "github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type bkEvent struct {
	nitro.HardModel
	Tenant string `json:"tenant"`
}

func (bkEvent) TableName() string { return "bk_events" }

func TestCountBucketsWhere_SingleQueryScoped(t *testing.T) {
	dsn := os.Getenv("NITRO_PG_DSN")
	if dsn == "" {
		t.Skip("set NITRO_PG_DSN")
	}
	ctx := context.Background()
	db, err := sqladapter.Open(ctx, "postgresql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Pgx().ExecSQL(ctx, `DROP TABLE IF EXISTS bk_events;
		CREATE TABLE bk_events (id uuid primary key, tenant text not null,
		created_at timestamptz not null, updated_at timestamptz not null)`); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Pgx().ExecSQL(ctx, `DROP TABLE IF EXISTS bk_events`) }()

	store := sqladapter.StoreFor[bkEvent](db)
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	add := func(tenant string, at time.Time) {
		e := &bkEvent{Tenant: tenant}
		if _, err := store.CreateOne(ctx, e); err != nil {
			t.Fatal(err)
		}
		_, err := store.UpdateWhere(ctx, store.Query().Where(blinqfield.Col[string]("bk_events", "id").Eq(e.ID.String())),
			blinqfield.Col[time.Time]("bk_events", "created_at").Set(at))
		if err != nil {
			t.Fatal(err)
		}
	}
	add("a", now.AddDate(0, 0, -2))
	add("a", now)
	add("a", now.Add(-time.Hour))
	add("b", now) // other tenant: must not count
	add("a", now.AddDate(0, 0, -10)) // outside the window

	got, err := store.CountBucketsWhere(ctx, blinqfield.Col[time.Time]("bk_events", "created_at").Ref(),
		query.DayBuckets(3, now), blinqfield.Col[string]("bk_events", "tenant").Eq("a"))
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{1, 0, 2}
	for i, g := range got {
		if g.Count != want[i] {
			t.Fatalf("bucket %d (%v) = %d, want %d; all=%+v", i, g.Key, g.Count, want[i], got)
		}
	}
}
