package pgx

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// OpenSQL returns *sql.DB over the pool with Exec/Query going through Gateway
// (use: sqlx.NewDb, GORM postgres.Config{Conn: ...}).
//
// Closing *sql.DB does NOT close the Pool.
func OpenSQL(pool *Pool, gw *gateway.Gateway) *sql.DB {
	inner := stdlib.GetPoolConnector(pool.DB())
	db := sql.OpenDB(observeConnector{inner: inner, gw: gw})
	db.SetMaxIdleConns(0)
	return db
}

// AdaptConnPool wraps *sql.DB for GORM (ConnPool) with Gateway observability.
func AdaptConnPool(gw *gateway.Gateway, db *sql.DB) *ConnPool {
	return &ConnPool{gw: gw, db: db}
}

// ConnPool implements the contract GORM uses (Prepare/Exec/Query/QueryRow Context).
type ConnPool struct {
	gw *gateway.Gateway
	db *sql.DB
}

func (p *ConnPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.db.PrepareContext(ctx, query)
}

func (p *ConnPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var res sql.Result
	err := p.gw.Instrument(ctx, gateway.InferOperation(query), query, argsAny(args), func() (int, error) {
		var err error
		res, err = p.db.ExecContext(ctx, query, args...)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		return int(n), nil
	})
	return res, err
}

func (p *ConnPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	var rows *sql.Rows
	err := p.gw.Instrument(ctx, "select", query, argsAny(args), func() (int, error) {
		var err error
		rows, err = p.db.QueryContext(ctx, query, args...)
		return 0, err
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (p *ConnPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	// *sql.Row cannot intercept Scan; uses observed QueryContext + fallback.
	// GORM typically uses QueryContext/ExecContext on ConnPool.
	return p.db.QueryRowContext(ctx, query, args...)
}

// --- driver.Connector wrapper (sqlx / database/sql) ---

type observeConnector struct {
	inner driver.Connector
	gw    *gateway.Gateway
}

func (c observeConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &observeConn{Conn: conn, gw: c.gw}, nil
}

func (c observeConnector) Driver() driver.Driver { return c.inner.Driver() }

type observeConn struct {
	driver.Conn
	gw *gateway.Gateway
}

func (c *observeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	anyArgs := namedToAny(args)
	var res driver.Result
	err := c.gw.Instrument(ctx, gateway.InferOperation(query), query, anyArgs, func() (int, error) {
		var err error
		res, err = execer.ExecContext(ctx, query, args)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		return int(n), nil
	})
	return res, err
}

func (c *observeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	anyArgs := namedToAny(args)
	var rows driver.Rows
	err := c.gw.Instrument(ctx, "select", query, anyArgs, func() (int, error) {
		var err error
		rows, err = queryer.QueryContext(ctx, query, args)
		return 0, err
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *observeConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if p, ok := c.Conn.(driver.ConnPrepareContext); ok {
		stmt, err := p.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
		return &observeStmt{Stmt: stmt, gw: c.gw, query: query}, nil
	}
	stmt, err := c.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &observeStmt{Stmt: stmt, gw: c.gw, query: query}, nil
}

type observeStmt struct {
	driver.Stmt
	gw    *gateway.Gateway
	query string
}

func (s *observeStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if e, ok := s.Stmt.(driver.StmtExecContext); ok {
		anyArgs := namedToAny(args)
		var res driver.Result
		err := s.gw.Instrument(ctx, gateway.InferOperation(s.query), s.query, anyArgs, func() (int, error) {
			var err error
			res, err = e.ExecContext(ctx, args)
			if err != nil {
				return 0, err
			}
			n, _ := res.RowsAffected()
			return int(n), nil
		})
		return res, err
	}
	return s.Stmt.Exec(namedToValue(args))
}

func (s *observeStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if q, ok := s.Stmt.(driver.StmtQueryContext); ok {
		anyArgs := namedToAny(args)
		var rows driver.Rows
		err := s.gw.Instrument(ctx, "select", s.query, anyArgs, func() (int, error) {
			var err error
			rows, err = q.QueryContext(ctx, args)
			return 0, err
		})
		return rows, err
	}
	return s.Stmt.Query(namedToValue(args))
}

func namedToAny(args []driver.NamedValue) []any {
	out := make([]any, len(args))
	for i, a := range args {
		out[i] = a.Value
	}
	return out
}

func namedToValue(args []driver.NamedValue) []driver.Value {
	out := make([]driver.Value, len(args))
	for i, a := range args {
		out[i] = a.Value
	}
	return out
}
