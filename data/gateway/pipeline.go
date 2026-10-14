package gateway

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Query runs SELECT through the pipeline: Cache → Trace → Execute → Metrics → Log → Cache Set.
//
// Only single-table statements are cached, keyed as db:select:<table>:<hash>,
// so writes to that table invalidate exactly the affected entries
// (multi-table / unattributable SQL is never cached — no stale reads).
func (g *Gateway) Query(ctx context.Context, sql string, args []any) ([]map[string]any, error) {
	start := time.Now()

	var table string
	if g.cache != nil {
		table = selectCacheTable(sql)
		if table != "" {
			key := selectKeyPrefix + table + ":" + queryHash(sql, args)
			var cached []map[string]any
			if err := g.cache.Get(key, &cached); err == nil && cached != nil {
				g.logQuery("select", 0, len(cached), true, len(args))
				return cached, nil
			}
		}
	}

	span := g.startSpan(ctx, "db.query")
	if span != nil {
		span.SetAttribute("db.operation", "select")
		span.SetAttribute("db.args_count", len(args))
		defer span.End()
	}

	rows, err := g.executor.Query(ctx, sql, args)
	duration := time.Since(start)
	if err != nil {
		g.recordError(span, "select", duration, err)
		return nil, err
	}
	defer rows.Close()

	cols := rows.Columns()
	var results []map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			g.recordError(span, "select", time.Since(start), err)
			return nil, err
		}
		row := make(map[string]any, len(values))
		for i, name := range cols {
			if i < len(values) {
				row[name] = values[i]
			}
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		g.recordError(span, "select", time.Since(start), err)
		return nil, err
	}

	duration = time.Since(start)
	g.recordMetrics("select", duration, len(results), false)
	g.logQuery("select", duration, len(results), false, len(args))

	if g.cache != nil && table != "" {
		_ = g.cache.Set(selectKeyPrefix+table+":"+queryHash(sql, args), results, g.selectTTL())
	}

	return results, nil
}

// QueryCursor exposes the raw SELECT cursor for direct row streaming
// (cursor→struct without intermediate maps). Tracing wraps acquisition;
// metrics/logging/Close remain the caller's responsibility.
//
// The result cache does not apply: typed consumers bypass cached []map rows.
func (g *Gateway) QueryCursor(ctx context.Context, sql string, args []any) (Rows, error) {
	span := g.startSpan(ctx, "db.query")
	if span != nil {
		span.SetAttribute("db.operation", "select")
		span.SetAttribute("db.args_count", len(args))
		defer span.End()
	}
	return g.executor.Query(ctx, sql, args)
}

// Exec runs INSERT/UPDATE/DELETE and returns rows affected.
func (g *Gateway) Exec(ctx context.Context, operation, sql string, args []any) (int64, error) {
	start := time.Now()

	span := g.startSpan(ctx, "db."+operation)
	if span != nil {
		span.SetAttribute("db.operation", operation)
		span.SetAttribute("db.args_count", len(args))
		defer span.End()
	}

	tag, err := g.executor.Exec(ctx, sql, args)
	duration := time.Since(start)
	if err != nil {
		g.recordError(span, operation, duration, err)
		return 0, err
	}

	n := tag.RowsAffected()
	g.recordMetrics(operation, duration, int(n), false)
	g.logQuery(operation, duration, int(n), false, len(args))

	if g.cache != nil && (operation == "insert" || operation == "update" || operation == "delete") {
		g.invalidateBySQL(sql)
	}

	return n, nil
}

// QueryRow runs a single-row query with the same observability.
// The span starts at Scan (pgx is lazy) so it is not orphaned if Scan is never called.
func (g *Gateway) QueryRow(ctx context.Context, sql string, args []any) Row {
	return &observedRow{
		g:     g,
		ctx:   ctx,
		sql:   sql,
		args:  args,
		start: time.Now(),
		row:   g.executor.QueryRow(ctx, sql, args),
	}
}

type observedRow struct {
	g     *Gateway
	ctx   context.Context
	sql   string
	args  []any
	start time.Time
	row   Row
}

func (o *observedRow) Scan(dest ...any) error {
	span := o.g.startSpan(o.ctx, "db.query_row")
	if span != nil {
		span.SetAttribute("db.operation", "query_row")
		span.SetAttribute("db.args_count", len(o.args))
		defer span.End()
	}
	err := o.row.Scan(dest...)
	duration := time.Since(o.start)
	if err != nil {
		o.g.recordError(span, "query_row", duration, err)
		return err
	}
	o.g.recordMetrics("query_row", duration, 1, false)
	o.g.logQuery("query_row", duration, 1, false, len(o.args))
	return nil
}

func (g *Gateway) startSpan(ctx context.Context, name string) domain.Span {
	if g.tracer == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, span := g.tracer.Start(ctx, name)
	return span
}

func cacheKey(operation, sql string, args []any) string {
	return "db:" + operation + ":" + queryHash(sql, args)
}

// queryHash fingerprints sql+args (FNV-64a) for cache keys.
func queryHash(sql string, args []any) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(sql))
	for _, a := range args {
		_, _ = h.Write([]byte{0})
		fmt.Fprintf(h, "%T:%v", a, a)
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// logQuery records execution metrics (no SQL / no query data).
func (g *Gateway) logQuery(operation string, duration time.Duration, rowCount int, cacheHit bool, argsCount int) {
	if g.logger == nil {
		return
	}
	attrs := []any{
		"operation", operation,
		"duration_ms", duration.Milliseconds(),
		"duration", duration.String(),
		"rows", rowCount,
		"args", argsCount,
		"cache_hit", cacheHit,
	}
	switch {
	case cacheHit:
		g.logger.Debug("db query completed (cache hit)", attrs...)
	case g.slow > 0 && duration >= g.slow:
		g.logger.Warn("db slow query", append(attrs, "slow_ms", g.slow.Milliseconds())...)
	default:
		// Per-query lines are noise at INFO; NITRO_LOG_LEVEL=debug shows them.
		g.logger.Debug("db query completed", attrs...)
	}
}

func (g *Gateway) recordError(span domain.Span, operation string, duration time.Duration, err error) {
	if span != nil {
		span.SetAttribute("db.error", err.Error())
		span.SetAttribute("db.operation", operation)
	}
	g.recordMetrics(operation, duration, 0, true)
	if g.logger != nil {
		g.logger.Error("db query failed",
			"operation", operation,
			"duration_ms", duration.Milliseconds(),
			"duration", duration.String(),
			"error", err,
		)
	}
}

func (g *Gateway) recordMetrics(operation string, duration time.Duration, rowCount int, failed bool) {
	if g.meter == nil {
		return
	}
	status := "ok"
	if failed {
		status = "error"
	}
	g.meter.Histogram("db.query.duration_ms").Record(float64(duration.Milliseconds()),
		"operation="+operation,
		"status="+status,
	)
	g.meter.Counter("db.query.count").Add(1,
		"operation="+operation,
		"status="+status,
	)
	if rowCount > 0 {
		g.meter.Histogram("db.query.rows").Record(float64(rowCount),
			"operation="+operation,
			"status="+status,
		)
	}
}
