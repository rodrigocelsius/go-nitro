package gateway

import (
	"context"
	"strings"
	"time"
)

// Instrument applies the same Exec/Query observability pipeline
// (span → metrics → log → cache invalidation on writes) around fn.
//
// Use for external libraries (GORM, sqlx, sqlc) that need native I/O
// (pgx.Rows / database/sql) but should go through Gateway.
func (g *Gateway) Instrument(ctx context.Context, operation, sql string, args []any, fn func() (rowCount int, err error)) error {
	if operation == "" {
		operation = InferOperation(sql)
	}
	start := time.Now()
	span := g.startSpan(ctx, "db."+operation)
	if span != nil {
		span.SetAttribute("db.operation", operation)
		span.SetAttribute("db.args_count", len(args))
		defer span.End()
	}

	n, err := fn()
	duration := time.Since(start)
	if err != nil {
		g.recordError(span, operation, duration, err)
		return err
	}

	g.recordMetrics(operation, duration, n, false)
	g.logQuery(operation, duration, n, false, len(args))

	if g.cache != nil && isWriteOp(operation) {
		g.invalidateBySQL(sql)
	}
	return nil
}

// InferOperation classifies simple SQL by the leading verb.
func InferOperation(sql string) string {
	s := strings.TrimSpace(sql)
	if len(s) < 6 {
		return "exec"
	}
	upper := strings.ToUpper(s[:6])
	switch {
	case strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(strings.ToUpper(s), "WITH"):
		return "select"
	case strings.HasPrefix(upper, "INSERT"):
		return "insert"
	case strings.HasPrefix(upper, "UPDATE"):
		return "update"
	case strings.HasPrefix(upper, "DELETE"):
		return "delete"
	default:
		return "exec"
	}
}

func isWriteOp(operation string) bool {
	switch operation {
	case "insert", "update", "delete", "exec":
		return true
	default:
		return false
	}
}
