//go:build integration

package sqladapter_test

import (
	"context"
	"os"
	"testing"

	blinqfield "github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type lkJob struct {
	nitro.HardModel
	Status string `json:"status"`
}

func (lkJob) TableName() string { return "lk_jobs" }

// Two concurrent consumers claim disjoint batches with FOR UPDATE SKIP LOCKED.
func TestSkipLocked_ConcurrentConsumersGetDisjointRows(t *testing.T) {
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
	if err := db.Pgx().ExecSQL(ctx, `DROP TABLE IF EXISTS lk_jobs;
		CREATE TABLE lk_jobs (id uuid primary key, status text not null, created_at timestamptz not null, updated_at timestamptz not null)`); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Pgx().ExecSQL(ctx, `DROP TABLE IF EXISTS lk_jobs`) }()
	store := sqladapter.StoreFor[lkJob](db)
	for i := 0; i < 6; i++ {
		if _, err := store.CreateOne(ctx, &lkJob{Status: "pending"}); err != nil {
			t.Fatal(err)
		}
	}
	status := blinqfield.Col[string]("lk_jobs", "status")
	created := blinqfield.Col[string]("lk_jobs", "created_at")

	held := make(chan []lkJob)
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- db.Tx(ctx, func(tx *sqladapter.DB) error {
			s := sqladapter.StoreFor[lkJob](tx)
			rows, err := s.Find(ctx, s.Query().Where(status.Eq("pending")).OrderBy(created.OrderAsc()).Limit(4).ForUpdate().SkipLocked())
			if err != nil {
				return err
			}
			held <- rows
			<-release
			return nil
		})
	}()
	first := <-held

	var second []lkJob
	if err := db.Tx(ctx, func(tx *sqladapter.DB) error {
		s := sqladapter.StoreFor[lkJob](tx)
		var err error
		second, err = s.Find(ctx, s.Query().Where(status.Eq("pending")).OrderBy(created.OrderAsc()).Limit(4).ForUpdate().SkipLocked())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if len(first) != 4 || len(second) != 2 {
		t.Fatalf("first=%d second=%d, want 4 and 2", len(first), len(second))
	}
	seen := map[string]bool{}
	for _, j := range first {
		seen[j.ID.String()] = true
	}
	for _, j := range second {
		if seen[j.ID.String()] {
			t.Fatalf("row %s claimed twice", j.ID)
		}
	}
}
