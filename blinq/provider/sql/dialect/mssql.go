package dialect

import (
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

type mssql struct{}

// MSSQL returns the Microsoft SQL Server dialect (@pN, [], OFFSET/FETCH; upsert via MERGE in compile).
func MSSQL() Dialect { return mssql{} }

func (mssql) Name() string { return "mssql" }
func (mssql) Placeholder(n int) string {
	return "@p" + itoa(n)
}
func (mssql) QuoteIdent(name string) string {
	return "[" + escapeIdent(name, ']') + "]"
}

// SelectTop uses TOP when there is a limit and no positive offset (OFFSET/FETCH needs ORDER BY).
func (mssql) SelectTop(limit, offset *uint64) string {
	if limit == nil {
		return ""
	}
	if offset != nil && *offset > 0 {
		return ""
	}
	return "TOP (" + uitoa(*limit) + ") "
}

func (mssql) LimitOffset(limit, offset *uint64) string {
	if offset == nil || *offset == 0 {
		// Limit-only (and offset 0) is expressed as TOP.
		return ""
	}
	s := " OFFSET " + uitoa(*offset) + " ROWS"
	if limit != nil {
		s += " FETCH NEXT " + uitoa(*limit) + " ROWS ONLY"
	}
	return s
}
func (mssql) ReturningSupported() bool { return false }
func (mssql) SoftDeleteNow() string    { return "GETDATE()" }
func (mssql) MaxPlaceholders() int     { return 2100 }
func (mssql) RenderILike(col, ph string) string {
	return "LOWER(" + col + ") LIKE LOWER(" + ph + ")"
}

// UpsertClause is unused for MSSQL — compile generates a full MERGE.
func (mssql) UpsertClause(q expr.Query) (string, error) {
	_ = q
	return "", fmt.Errorf("blinq/sql: mssql upsert uses MERGE (handled in compile)")
}
