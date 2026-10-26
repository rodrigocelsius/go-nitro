package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// txTestExec counts statements per executor so tests can tell whether a call
// ran on the pool or on the transaction.
type txTestExec struct {
	testExec
	name      string
	begins    int
	commits   int
	rollbacks int
	tx        *txTestExec
	commitErr error
}

func (e *txTestExec) BeginTx(context.Context) (TxExecutor, error) {
	e.begins++
	e.tx = &txTestExec{name: "tx", commitErr: e.commitErr}
	e.tx.tx = e // back-pointer for counters
	return e.tx, nil
}

func (e *txTestExec) Commit(context.Context) error {
	e.tx.commits++
	return e.commitErr
}

func (e *txTestExec) Rollback(context.Context) error {
	e.tx.rollbacks++
	return nil
}

func TestInTx_RoutesCallsToTransactionAndCommits(t *testing.T) {
	pool := &txTestExec{name: "pool"}
	g := New(pool, DisableObservability())
	ctx := context.Background()

	err := g.InTx(ctx, func(ctx context.Context) error {
		if _, err := g.Exec(ctx, "insert", "INSERT INTO a VALUES (1)", nil); err != nil {
			return err
		}
		// Nested InTx joins: no second BEGIN.
		return g.InTx(ctx, func(ctx context.Context) error {
			_, err := g.Exec(ctx, "insert", "INSERT INTO b VALUES (1)", nil)
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if pool.begins != 1 || pool.commits != 1 || pool.rollbacks != 0 {
		t.Fatalf("begins=%d commits=%d rollbacks=%d", pool.begins, pool.commits, pool.rollbacks)
	}
	if pool.writes != 0 || pool.tx.writes != 2 {
		t.Fatalf("pool writes=%d tx writes=%d", pool.writes, pool.tx.writes)
	}
	// Outside the tx calls go to the pool again.
	_, _ = g.Exec(ctx, "insert", "INSERT INTO a VALUES (2)", nil)
	if pool.writes != 1 {
		t.Fatalf("pool writes=%d, want 1", pool.writes)
	}
}

func TestInTx_RollbackOnErrorAndPanic(t *testing.T) {
	pool := &txTestExec{name: "pool"}
	g := New(pool, DisableObservability())
	boom := errors.New("boom")
	if err := g.InTx(context.Background(), func(context.Context) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if pool.rollbacks != 1 || pool.commits != 0 {
		t.Fatalf("rollbacks=%d commits=%d", pool.rollbacks, pool.commits)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic must propagate")
			}
		}()
		_ = g.InTx(context.Background(), func(context.Context) error { panic("x") })
	}()
	if pool.rollbacks != 2 {
		t.Fatalf("rollbacks=%d after panic", pool.rollbacks)
	}
}

type sqlStateErr string

func (e sqlStateErr) Error() string    { return string(e) }
func (e sqlStateErr) SQLState() string { return string(e) }

func TestInTx_RetryableCommitBecomesTxConflict(t *testing.T) {
	pool := &txTestExec{name: "pool", commitErr: sqlStateErr("40001")}
	g := New(pool, DisableObservability())
	err := g.InTx(context.Background(), func(context.Context) error { return nil })
	var conflict *domain.TxConflictError
	if !errors.As(err, &conflict) || !domain.IsRetryable(err) {
		t.Fatalf("err = %T %v, want TxConflictError", err, err)
	}
}

func TestInTx_NoTxSupport(t *testing.T) {
	g := New(&testExec{}, DisableObservability())
	if err := g.InTx(context.Background(), func(context.Context) error { return nil }); !errors.Is(err, ErrNoTxSupport) {
		t.Fatalf("err = %v", err)
	}
}

func TestInTx_BypassesSelectCacheAndPin(t *testing.T) {
	pool := &txTestExec{name: "pool"}
	mem := newTestCache()
	g := New(pool, WithCache(mem))
	sql := `SELECT * FROM "users"`
	_ = g.InTx(context.Background(), func(ctx context.Context) error {
		if _, err := g.Query(ctx, sql, nil); err != nil {
			return err
		}
		if len(mem.items) != 0 {
			t.Fatal("SELECT inside a tx must not be cached")
		}
		pinned := g.Pin(ctx)
		if pinned == g {
			t.Fatal("Pin inside a tx must return a tx-bound gateway")
		}
		// Pinned gateway works with any ctx and joins on nested InTx.
		return pinned.InTx(context.Background(), func(ctx context.Context) error {
			_, err := pinned.Exec(ctx, "insert", "INSERT INTO users VALUES (1)", nil)
			return err
		})
	})
	if pool.begins != 1 || pool.tx.writes != 1 {
		t.Fatalf("begins=%d tx writes=%d", pool.begins, pool.tx.writes)
	}
	if g.Pin(context.Background()) != g {
		t.Fatal("Pin outside a tx must return the gateway itself")
	}
	if g.TxFrom(context.Background()) != nil {
		t.Fatal("TxFrom outside a tx must be nil")
	}
}

func TestExtractTable_IrregularSpacing(t *testing.T) {
	cases := map[string]string{
		"INSERT\n   INTO orders (id) VALUES ($1)": "orders",
		"DELETE  FROM \"users\" WHERE id = $1":    "users",
		"INSERT IGNORE INTO t VALUES (1)":         "",
	}
	for sql, want := range cases {
		if got := extractTable(sql); got != want {
			t.Errorf("extractTable(%q) = %q, want %q", sql, got, want)
		}
	}
}
