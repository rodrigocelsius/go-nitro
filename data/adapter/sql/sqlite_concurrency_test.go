package sqladapter_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
)

// Concurrent writers on a SQLite file must queue on the lock (busy_timeout +
// WAL by default) instead of failing with "database is locked".
func TestSQLiteFile_ConcurrentWritersDoNotFail(t *testing.T) {
	ctx := context.Background()
	db, err := sqladapter.Open(ctx, "sqlite", "file:"+filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := execRaw(ctx, db, txSchema); err != nil {
		t.Fatal(err)
	}
	sales := sqladapter.StoreFor[TxSale](db)
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- db.InTx(ctx, func(ctx context.Context) error {
				_, err := sales.CreateOne(ctx, &TxSale{Customer: "c"})
				return err
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent write failed: %v", err)
		}
	}
	if n := countRows[TxSale](t, db); n != 64 {
		t.Fatalf("rows = %d, want 64", n)
	}
}
