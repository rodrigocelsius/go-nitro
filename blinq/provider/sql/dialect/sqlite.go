package dialect

import (
	"fmt"
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

type sqlite struct{}

// SQLite returns the SQLite dialect (?, LOWER LIKE, ON CONFLICT; optional RETURNING off).
func SQLite() Dialect { return sqlite{} }

func (sqlite) Name() string             { return "sqlite" }
func (sqlite) Placeholder(n int) string { return "?" }
func (sqlite) QuoteIdent(name string) string {
	return `"` + escapeIdent(name, '"') + `"`
}
func (sqlite) SelectTop(limit, offset *uint64) string { return "" }
func (sqlite) LimitOffset(limit, offset *uint64) string {
	return limitOffsetSQL(limit, offset)
}
func (sqlite) ReturningSupported() bool { return false }
func (sqlite) SoftDeleteNow() string    { return "CURRENT_TIMESTAMP" }
func (sqlite) MaxPlaceholders() int     { return 999 }
func (sqlite) RenderILike(col, ph string) string {
	return "LOWER(" + col + ") LIKE LOWER(" + ph + ")"
}

func (s sqlite) UpsertClause(q expr.Query) (string, error) {
	c := q.Conflict
	if c == nil || len(c.Columns) == 0 {
		return "", errConflictRequired
	}
	var sb strings.Builder
	sb.WriteString(" ON CONFLICT (")
	for i, col := range c.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(s.QuoteIdent(col.Column))
	}
	sb.WriteString(")")
	switch c.Action {
	case expr.ConflictDoNothing:
		sb.WriteString(" DO NOTHING")
	case expr.ConflictDoUpdate:
		sets, err := BuildDoUpdateSets(s, q)
		if err != nil {
			return "", err
		}
		sb.WriteString(" DO UPDATE SET ")
		sb.WriteString(sets)
	default:
		return "", fmt.Errorf("blinq/sql: unknown conflict action")
	}
	return sb.String(), nil
}
