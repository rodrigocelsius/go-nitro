package pgx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rodrigocelsius/go-nitro/data/gateway/v2"
)

// DBTX is the sqlc (pgx) contract — Exec/Query/QueryRow go through Gateway.
type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

// AdaptDBTX adapts pool + Gateway to the sqlc DBTX contract.
func AdaptDBTX(gw *gateway.Gateway, pool *Pool) DBTX {
	return &gatewayDBTX{gw: gw, pool: pool}
}

type gatewayDBTX struct {
	gw   *gateway.Gateway
	pool *Pool
}

func (d *gatewayDBTX) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	var tag pgconn.CommandTag
	err := d.gw.Instrument(ctx, gateway.InferOperation(sql), sql, argsAny(args), func() (int, error) {
		var err error
		tag, err = d.pool.DB().Exec(ctx, sql, args...)
		if err != nil {
			return 0, err
		}
		return int(tag.RowsAffected()), nil
	})
	return tag, err
}

func (d *gatewayDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	var rows pgx.Rows
	err := d.gw.Instrument(ctx, "select", sql, argsAny(args), func() (int, error) {
		var err error
		rows, err = d.pool.DB().Query(ctx, sql, args...)
		return 0, err
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (d *gatewayDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return &gatewayPgxRow{
		gw:   d.gw,
		sql:  sql,
		args: argsAny(args),
		row:  d.pool.DB().QueryRow(ctx, sql, args...),
		ctx:  ctx,
	}
}

type gatewayPgxRow struct {
	gw   *gateway.Gateway
	ctx  context.Context
	sql  string
	args []any
	row  pgx.Row
}

func (r *gatewayPgxRow) Scan(dest ...any) error {
	return r.gw.Instrument(r.ctx, "query_row", r.sql, r.args, func() (int, error) {
		if err := r.row.Scan(dest...); err != nil {
			return 0, err
		}
		return 1, nil
	})
}

func argsAny(args []interface{}) []any {
	out := make([]any, len(args))
	copy(out, args)
	return out
}
