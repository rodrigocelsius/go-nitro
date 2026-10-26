package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// TxBeginner is implemented by executors that can open a transaction
// (pgx pool, *sql.DB). Gateway.InTx requires it.
type TxBeginner interface {
	BeginTx(ctx context.Context) (TxExecutor, error)
}

// TxExecutor is a SQLExecutor bound to one open transaction.
type TxExecutor interface {
	SQLExecutor
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// ErrNoTxSupport is returned by InTx when the executor cannot begin a transaction.
var ErrNoTxSupport = errors.New("gateway: executor does not support transactions")

// txKey scopes the bound transaction to one root Gateway, so a transaction on
// database A never captures queries for database B made with the same ctx.
type txKey struct{ root *Gateway }

func (g *Gateway) rootGateway() *Gateway {
	if g.root != nil {
		return g.root
	}
	return g
}

// txFrom returns the transaction of g bound to ctx, if any.
func (g *Gateway) txFrom(ctx context.Context) TxExecutor {
	if ctx == nil {
		return nil
	}
	tx, _ := ctx.Value(txKey{g.rootGateway()}).(TxExecutor)
	return tx
}

// exec picks the executor for ctx: the ambient transaction of this database
// when there is one, else the gateway's own executor.
func (g *Gateway) exec(ctx context.Context) SQLExecutor {
	if tx := g.txFrom(ctx); tx != nil {
		return tx
	}
	return g.executor
}

// inTx reports whether this call runs inside a transaction (ambient or pinned).
func (g *Gateway) inTx(ctx context.Context) bool {
	return g.root != nil || g.txFrom(ctx) != nil
}

// InTx runs fn inside a transaction. Every Query/Exec on this Gateway — and on
// every Store built from it — made with the ctx passed to fn joins the
// transaction. A nested InTx on the same database joins the outer one (no
// savepoint): the outermost call commits or rolls back.
//
// fn returning an error or panicking rolls back; transient failures
// (serialization / deadlock, see domain.IsRetryable) are returned as
// *domain.TxConflictError so callers (and Resource → HTTP 409) can retry.
func (g *Gateway) InTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if g == nil {
		return fmt.Errorf("gateway: InTx on nil Gateway")
	}
	if fn == nil {
		return fmt.Errorf("gateway: InTx requires fn")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if g.inTx(ctx) {
		return fn(ctx) // join the enclosing transaction
	}
	root := g.rootGateway()
	beginner, ok := root.executor.(TxBeginner)
	if !ok {
		return ErrNoTxSupport
	}
	tx, err := beginner.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("gateway: begin: %w", err)
	}
	txCtx, scope, owner := domain.BeginTxScope(ctx)
	txCtx = context.WithValue(txCtx, txKey{root}, tx)

	finish := func(committed bool) {
		if !owner {
			return
		}
		if committed {
			scope.Commit(ctx)
		} else {
			scope.Rollback(ctx)
		}
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx) // best-effort: the panic must propagate
			finish(false)
			panic(p)
		}
	}()

	if err = fn(txCtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			err = fmt.Errorf("%w (rollback also failed: %v)", err, rbErr)
		}
		finish(false)
		return wrapRetryable(err)
	}
	if err = tx.Commit(ctx); err != nil {
		_ = tx.Rollback(ctx)
		finish(false)
		return wrapRetryable(fmt.Errorf("gateway: commit: %w", err))
	}
	finish(true)
	return nil
}

func wrapRetryable(err error) error {
	var conflict *domain.TxConflictError
	if domain.IsRetryable(err) && !errors.As(err, &conflict) {
		return domain.NewTxConflict(err)
	}
	return err
}

// Pin returns a Gateway that runs every call on the transaction bound to ctx
// (used by sqladapter.DB.Tx so StoreOf(tx, …) works with any ctx). Returns g
// unchanged when ctx carries no transaction for this database.
func (g *Gateway) Pin(ctx context.Context) *Gateway {
	tx := g.txFrom(ctx)
	if tx == nil {
		return g
	}
	cp := *g
	cp.executor = tx
	cp.root = g.rootGateway()
	return &cp
}

// TxFrom exposes the transaction executor of this database bound to ctx
// (nil outside InTx). Escape hatch for raw SQL inside a unit of work.
func (g *Gateway) TxFrom(ctx context.Context) SQLExecutor {
	if tx := g.txFrom(ctx); tx != nil {
		return tx
	}
	return nil
}
