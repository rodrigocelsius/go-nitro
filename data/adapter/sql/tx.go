package sqladapter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// Tx runs fn inside a database transaction.
// StoreOf(txDB, …) uses the same transaction. On success the tx is committed;
// on error or panic it is rolled back.
//
//	err := db.Tx(ctx, func(tx *sqladapter.DB) error {
//	    users := sqladapter.StoreOf[User](tx, "users")
//	    orders := sqladapter.StoreOf[Order](tx, "orders")
//	    u, err := users.CreateOne(ctx, &User{Name: "Ada"})
//	    if err != nil { return err }
//	    _, err = orders.CreateOne(ctx, &Order{UserID: u.ID, Total: 10})
//	    return err
//	})
func (d *DB) Tx(ctx context.Context, fn func(tx *DB) error) (err error) {
	if d == nil {
		return fmt.Errorf("sqladapter: Tx on nil DB")
	}
	if fn == nil {
		return fmt.Errorf("sqladapter: Tx requires fn")
	}

	var (
		commit   func(context.Context) error
		rollback func(context.Context) error
		txDB     *DB
	)

	if d.pg != nil {
		exec := pgxadapter.NewExecutor(d.pg.Pool)
		pgTx, beginErr := exec.Begin(ctx)
		if beginErr != nil {
			return fmt.Errorf("sqladapter: begin: %w", beginErr)
		}
		commit = pgTx.Commit
		rollback = pgTx.Rollback
		gw := gateway.New(pgxadapter.AdaptTx(pgTx), withEnvQuiet(nil)...)
		txDB = &DB{Driver: d.Driver, dialect: d.dialect, gw: gw}
	} else if d.std != nil {
		sqlTx, beginErr := d.std.BeginTx(ctx, nil)
		if beginErr != nil {
			return fmt.Errorf("sqladapter: begin: %w", beginErr)
		}
		commit = func(c context.Context) error { return sqlTx.Commit() }
		rollback = func(c context.Context) error { return sqlTx.Rollback() }
		gw := gateway.New(AdaptStdTx(sqlTx), withEnvQuiet(nil)...)
		txDB = &DB{Driver: d.Driver, dialect: d.dialect, gw: gw, std: nil}
	} else {
		return fmt.Errorf("sqladapter: no pool")
	}

	defer func() {
		if p := recover(); p != nil {
			// Rollback best-effort: the original panic must propagate.
			_ = rollback(ctx)
			panic(p)
		}
	}()

	if err = fn(txDB); err != nil {
		if rbErr := rollback(ctx); rbErr != nil {
			return fmt.Errorf("sqladapter: %w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}
	if err = commit(ctx); err != nil {
		if rbErr := rollback(ctx); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return fmt.Errorf("sqladapter: commit: %w (rollback also failed: %v)", err, rbErr)
		}
		return fmt.Errorf("sqladapter: commit: %w", err)
	}
	return nil
}
