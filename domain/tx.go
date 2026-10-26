package domain

import (
	"context"
	"errors"
	"log"
	"sync"
)

// Transactor runs fn as one unit of work (a database transaction).
//
// Every call made with the ctx handed to fn joins the same transaction — a
// BLinq Store, a Resource repository or a nested InTx on the same database —
// so hooks and use cases compose without passing a tx handle around. fn
// returning an error (or panicking) rolls everything back; nil commits.
//
// Implemented by sqladapter.DB, pgx.DB and every BLinq Store.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// TxScope tracks the callbacks of one outermost transaction. Transactor
// implementations create it with BeginTxScope and finish it with Commit or
// Rollback; application code only uses AfterCommit / AfterRollback / InTx.
type TxScope struct {
	mu         sync.Mutex
	afterOK    []func(context.Context)
	afterAbort []func(context.Context)
	done       bool
}

type txScopeKey struct{}

// BeginTxScope returns ctx carrying a scope for a new transaction. When ctx
// already carries one (nested transaction on another database, or a joined
// call), the outer scope is reused and owner is false: only the owner runs
// the callbacks, once the outermost transaction finishes.
func BeginTxScope(ctx context.Context) (txCtx context.Context, scope *TxScope, owner bool) {
	if s, ok := ctx.Value(txScopeKey{}).(*TxScope); ok && s != nil {
		return ctx, s, false
	}
	s := &TxScope{}
	return context.WithValue(ctx, txScopeKey{}, s), s, true
}

// InTx reports whether ctx is inside a transaction started by a Transactor.
func InTx(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	s, ok := ctx.Value(txScopeKey{}).(*TxScope)
	return ok && s != nil
}

// AfterCommit runs fn once the outermost transaction in ctx commits — the
// place for side effects that must not happen if the writes are rolled back
// (publish a queue message, send an e-mail, bust a shared cache). Outside a
// transaction fn runs immediately. A panic in fn is logged, never propagated:
// the data is already committed.
func AfterCommit(ctx context.Context, fn func(ctx context.Context)) {
	if fn == nil {
		return
	}
	if s, ok := ctx.Value(txScopeKey{}).(*TxScope); ok && s != nil {
		s.mu.Lock()
		if !s.done {
			s.afterOK = append(s.afterOK, fn)
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
	}
	runTxCallback(ctx, fn)
}

// AfterRollback runs fn if the outermost transaction in ctx rolls back
// (compensation, metrics, alerting). Outside a transaction it is a no-op.
func AfterRollback(ctx context.Context, fn func(ctx context.Context)) {
	if fn == nil {
		return
	}
	if s, ok := ctx.Value(txScopeKey{}).(*TxScope); ok && s != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		if !s.done {
			s.afterAbort = append(s.afterAbort, fn)
		}
	}
}

// Commit marks the scope committed and runs AfterCommit callbacks in
// registration order. ctx should be the caller's ctx (outside the tx).
func (s *TxScope) Commit(ctx context.Context) { s.finish(ctx, true) }

// Rollback marks the scope aborted and runs AfterRollback callbacks.
func (s *TxScope) Rollback(ctx context.Context) { s.finish(ctx, false) }

func (s *TxScope) finish(ctx context.Context, ok bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.done {
		s.mu.Unlock()
		return
	}
	s.done = true
	fns := s.afterAbort
	if ok {
		fns = s.afterOK
	}
	s.afterOK, s.afterAbort = nil, nil
	s.mu.Unlock()
	for _, fn := range fns {
		runTxCallback(ctx, fn)
	}
}

func runTxCallback(ctx context.Context, fn func(context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("nitro: transaction callback panic: %v", r)
		}
	}()
	fn(ctx)
}

// RunInTx runs fn through t when t is a Transactor, or directly otherwise
// (memory repositories, mocks). It is how Resource makes its write path
// atomic without requiring every repository to support transactions.
func RunInTx(ctx context.Context, t any, fn func(ctx context.Context) error) error {
	if tx, ok := t.(Transactor); ok && tx != nil {
		return tx.InTx(ctx, fn)
	}
	return fn(ctx)
}

// TxConflictError is a transient transaction failure (serialization failure,
// deadlock, lock timeout): the whole unit of work can be retried as is.
// It maps to HTTP 409 with code "tx_conflict".
type TxConflictError struct {
	Err error
}

func (e *TxConflictError) Error() string {
	if e.Err == nil {
		return "transaction conflict, retry"
	}
	return "transaction conflict, retry: " + e.Err.Error()
}
func (e *TxConflictError) StatusCode() int { return 409 }
func (e *TxConflictError) Body() any {
	return map[string]any{"error": "transaction conflict, retry", "code": "tx_conflict", "retryable": true}
}
func (e *TxConflictError) Unwrap() error   { return e.Err }
func (e *TxConflictError) Retryable() bool { return true }

// NewTxConflict wraps err as a retryable transaction conflict.
func NewTxConflict(err error) error { return &TxConflictError{Err: err} }

var (
	retryableMu     sync.RWMutex
	retryableChecks []func(error) bool
)

// RegisterRetryable adds a driver-specific transient-error check (e.g. MySQL
// 1213 deadlock / 1205 lock wait timeout, registered by sqladapter).
func RegisterRetryable(check func(error) bool) {
	if check == nil {
		return
	}
	retryableMu.Lock()
	retryableChecks = append(retryableChecks, check)
	retryableMu.Unlock()
}

// IsRetryable reports whether err (or anything it wraps) is a transient
// transaction failure: a TxConflictError, an error with Retryable() true, a
// driver error with SQLSTATE 40001 (serialization) / 40P01 (deadlock), or a
// check added with RegisterRetryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	retryableMu.RLock()
	checks := retryableChecks
	retryableMu.RUnlock()
	for _, check := range checks {
		if check(err) {
			return true
		}
	}
	var r interface{ Retryable() bool }
	if errors.As(err, &r) && r.Retryable() {
		return true
	}
	var st interface{ SQLState() string }
	if errors.As(err, &st) {
		switch st.SQLState() {
		case "40001", "40P01":
			return true
		}
	}
	return false
}
