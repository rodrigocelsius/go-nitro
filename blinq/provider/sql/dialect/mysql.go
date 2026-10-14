package dialect

import (
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

type mysql struct{}

// MySQL returns the MySQL/MariaDB dialect (?, backticks, ON DUPLICATE KEY; no RETURNING).
func MySQL() Dialect { return mysql{} }

func (mysql) Name() string             { return "mysql" }
func (mysql) Placeholder(n int) string { return "?" }
func (mysql) QuoteIdent(name string) string {
	return "`" + escapeIdent(name, '`') + "`"
}
func (mysql) SelectTop(limit, offset *uint64) string { return "" }
func (mysql) LimitOffset(limit, offset *uint64) string {
	return limitOffsetSQL(limit, offset)
}
func (mysql) ReturningSupported() bool { return false }
func (mysql) SoftDeleteNow() string    { return "CURRENT_TIMESTAMP" }
func (mysql) MaxPlaceholders() int     { return 65535 }
func (mysql) RenderILike(col, ph string) string {
	return "LOWER(" + col + ") LIKE LOWER(" + ph + ")"
}

func (m mysql) UpsertClause(q expr.Query) (string, error) {
	c := q.Conflict
	if c == nil {
		return "", errConflictRequired
	}
	// MySQL uses a UNIQUE index; OnConflict columns are documentary / validated.
	if len(c.Columns) == 0 {
		return "", errConflictRequired
	}
	switch c.Action {
	case expr.ConflictDoNothing:
		// Emulates DO NOTHING: updating a column to itself (no-op) requires UNIQUE.
		// We prefer an explicit error — MySQL 8.0.19+ has an alias, but DO NOTHING does not exist.
		return "", fmt.Errorf("blinq/sql: DoNothing unsupported on mysql (use unique insert ignore or DoUpdate)")
	case expr.ConflictDoUpdate:
		sets, err := BuildDoUpdateSets(m, q)
		if err != nil {
			return "", err
		}
		return " ON DUPLICATE KEY UPDATE " + sets, nil
	default:
		return "", fmt.Errorf("blinq/sql: unknown conflict action")
	}
}
