package dialect

import (
	"fmt"
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

type postgres struct{}

// Postgres returns the PostgreSQL dialect ($N, ILIKE, RETURNING, ON CONFLICT).
func Postgres() Dialect { return postgres{} }

func (postgres) Name() string { return "postgres" }
func (postgres) Placeholder(n int) string {
	return "$" + itoa(n)
}
func (postgres) QuoteIdent(name string) string {
	return `"` + escapeIdent(name, '"') + `"`
}
func (postgres) SelectTop(limit, offset *uint64) string { return "" }
func (postgres) LimitOffset(limit, offset *uint64) string {
	return limitOffsetSQL(limit, offset)
}
func (postgres) ReturningSupported() bool { return true }
func (postgres) SoftDeleteNow() string    { return "NOW()" }
func (postgres) MaxPlaceholders() int     { return 65535 }
func (postgres) RenderILike(col, ph string) string {
	return col + " ILIKE " + ph
}

func (p postgres) UpsertClause(q expr.Query) (string, error) {
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
		sb.WriteString(p.QuoteIdent(col.Column))
	}
	sb.WriteString(")")
	switch c.Action {
	case expr.ConflictDoNothing:
		sb.WriteString(" DO NOTHING")
	case expr.ConflictDoUpdate:
		sets, err := BuildDoUpdateSets(p, q)
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
