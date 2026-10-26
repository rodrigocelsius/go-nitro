package domain

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type fakeSQLState string

func (f fakeSQLState) Error() string    { return "sqlstate " + string(f) }
func (f fakeSQLState) SQLState() string { return string(f) }

func TestAfterCommit_RunsImmediatelyOutsideTx(t *testing.T) {
	ran := false
	AfterCommit(context.Background(), func(context.Context) { ran = true })
	if !ran {
		t.Fatal("AfterCommit outside a tx must run immediately")
	}
	AfterRollback(context.Background(), func(context.Context) { t.Fatal("AfterRollback outside a tx must not run") })
}

func TestTxScope_CommitAndRollback(t *testing.T) {
	ctx, scope, owner := BeginTxScope(context.Background())
	if !owner || !InTx(ctx) {
		t.Fatal("expected a new owned scope")
	}
	_, same, nestedOwner := BeginTxScope(ctx)
	if same != scope || nestedOwner {
		t.Fatal("nested BeginTxScope must reuse the outer scope")
	}
	var order []string
	AfterCommit(ctx, func(context.Context) { order = append(order, "a") })
	AfterCommit(ctx, func(context.Context) { panic("ignored") })
	AfterCommit(ctx, func(context.Context) { order = append(order, "b") })
	AfterRollback(ctx, func(context.Context) { order = append(order, "rb") })
	scope.Commit(context.Background())
	scope.Rollback(context.Background()) // already finished: no-op
	if fmt.Sprint(order) != "[a b]" {
		t.Fatalf("order = %v", order)
	}
	// Registering after the scope finished runs immediately.
	AfterCommit(ctx, func(context.Context) { order = append(order, "late") })
	if order[len(order)-1] != "late" {
		t.Fatal("late AfterCommit should run immediately")
	}

	ctx2, scope2, _ := BeginTxScope(context.Background())
	rolled := false
	AfterCommit(ctx2, func(context.Context) { t.Fatal("commit callback on rollback") })
	AfterRollback(ctx2, func(context.Context) { rolled = true })
	scope2.Rollback(context.Background())
	if !rolled {
		t.Fatal("AfterRollback did not run")
	}
}

type fakeTransactor struct{ calls int }

func (f *fakeTransactor) InTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(ctx)
}

func TestRunInTx(t *testing.T) {
	f := &fakeTransactor{}
	if err := RunInTx(context.Background(), f, func(context.Context) error { return nil }); err != nil || f.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, f.calls)
	}
	ran := false
	_ = RunInTx(context.Background(), struct{}{}, func(context.Context) error { ran = true; return nil })
	if !ran {
		t.Fatal("non-Transactor must run fn directly")
	}
	_ = RunInTx(context.Background(), nil, func(context.Context) error { return nil })
}

func TestIsRetryable(t *testing.T) {
	cases := map[error]bool{
		nil:                   false,
		errors.New("x"):       false,
		fakeSQLState("40001"): true,
		fakeSQLState("40P01"): true,
		fakeSQLState("23505"): false,
		fmt.Errorf("w: %w", fakeSQLState("40001")): true,
		NewTxConflict(errors.New("deadlock")):      true,
	}
	for err, want := range cases {
		if got := IsRetryable(err); got != want {
			t.Errorf("IsRetryable(%v) = %v, want %v", err, got, want)
		}
	}
	var he HTTPError = &TxConflictError{}
	if he.StatusCode() != 409 || he.Error() == "" {
		t.Fatal("TxConflictError must be a 409 HTTPError")
	}
	if !errors.Is(NewTxConflict(fakeSQLState("40001")), fakeSQLState("40001")) {
		t.Fatal("TxConflictError must unwrap")
	}
}
