package dialect

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

// Dialect abstracts SQL differences across databases.
type Dialect interface {
	Name() string
	Placeholder(n int) string
	QuoteIdent(name string) string
	// SelectTop is inserted after SELECT [DISTINCT] (e.g. MSSQL "TOP (n) ").
	// Empty when LimitOffset handles pagination (Postgres/MySQL/SQLite).
	SelectTop(limit, offset *uint64) string
	LimitOffset(limit, offset *uint64) string
	ReturningSupported() bool
	SoftDeleteNow() string
	// RenderILike generates a case-insensitive comparison (ILIKE or LOWER LIKE).
	RenderILike(col, placeholder string) string
	// UpsertClause generates ON CONFLICT / ON DUPLICATE KEY (no RETURNING).
	UpsertClause(q expr.Query) (string, error)
	// MaxPlaceholders is the safe bind-parameter budget for a single statement
	// (driver/server limit minus a small margin). Used to auto-chunk bulk inserts.
	MaxPlaceholders() int
}

// escapeIdent doubles quote/bracket characters inside an identifier.
func escapeIdent(name string, quote rune) string {
	q := string(quote)
	return strings.ReplaceAll(name, q, q+q)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func uitoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func limitOffsetSQL(limit, offset *uint64) string {
	s := ""
	if limit != nil {
		s += " LIMIT " + uitoa(*limit)
	}
	if offset != nil {
		s += " OFFSET " + uitoa(*offset)
	}
	return s
}

// ExcludedRef references the proposed upsert row (EXCLUDED / excluded / VALUES / source).
// Column names are always QuoteIdent'd for consistent quoting across dialects.
func ExcludedRef(d Dialect, col string) string {
	q := d.QuoteIdent(col)
	switch d.Name() {
	case "mysql":
		return "VALUES(" + q + ")"
	case "sqlite":
		return "excluded." + q
	case "mssql":
		return "[source]." + q
	default: // postgres
		return "EXCLUDED." + q
	}
}

// BuildDoUpdateSets builds "col = ref, …" for DO UPDATE / ON DUPLICATE.
func BuildDoUpdateSets(d Dialect, q expr.Query) (string, error) {
	c := q.Conflict
	if c == nil {
		return "", errConflictRequired
	}
	sets := c.Updates
	if len(sets) == 0 {
		conflictCols := map[string]bool{}
		for _, col := range c.Columns {
			conflictCols[col.Column] = true
		}
		var parts []string
		for _, col := range q.Columns {
			if conflictCols[col] {
				continue
			}
			parts = append(parts, d.QuoteIdent(col)+" = "+ExcludedRef(d, col))
		}
		if len(parts) == 0 {
			return "", errNoUpdateCols
		}
		return joinComma(parts), nil
	}
	parts := make([]string, len(sets))
	for i, s := range sets {
		parts[i] = d.QuoteIdent(s.Field.Column) + " = " + ExcludedRef(d, s.Field.Column)
	}
	return joinComma(parts), nil
}

func joinComma(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	n := len(parts) - 1
	for _, p := range parts {
		n += len(p)
	}
	b := make([]byte, 0, n)
	for i, p := range parts {
		if i > 0 {
			b = append(b, ',', ' ')
		}
		b = append(b, p...)
	}
	return string(b)
}

var (
	errConflictRequired = errStr("blinq/sql: upsert requires OnConflict columns")
	errNoUpdateCols     = errStr("blinq/sql: DoUpdate has no columns to update")
)

type errStr string

func (e errStr) Error() string { return string(e) }
