package gateway

import (
	"strings"
)

// selectKeyPrefix namespaces cached SELECT entries as db:select:<table>:<hash>.
// Write statements invalidate per table, so every cached SELECT must embed the
// (single) table it reads from; multi-table statements are not cached at all.
const selectKeyPrefix = "db:select:"

// invalidateBySQL invalidates the cached SELECT keys of every table touched by
// a write statement (INSERT / UPDATE / DELETE / MERGE).
func (g *Gateway) invalidateBySQL(sql string) {
	if g.cache == nil {
		return
	}
	for _, t := range writeTables(sql) {
		_ = g.cache.Invalidate(selectKeyPrefix + t + ":*")
	}
}

// extractTable returns the primary target of a write statement
// (INSERT INTO / UPDATE / DELETE FROM / MERGE [INTO]), unquoted.
// Returns "" for non-write or unrecognized SQL.
func extractTable(sql string) string {
	ts := writeTables(sql)
	if len(ts) == 0 {
		return ""
	}
	return ts[0]
}

// writeTables returns the target table of a write statement based on its
// leading verb (INSERT / UPDATE / DELETE / MERGE), so later clauses that
// merely mention another keyword (e.g. MERGE … WHEN MATCHED THEN UPDATE SET)
// never produce phantom tables.
func writeTables(sql string) []string {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return nil
	}
	upper := strings.ToUpper(trimmed)
	var kw string
	switch {
	case strings.HasPrefix(upper, "INSERT"):
		kw = "INSERT INTO "
	case strings.HasPrefix(upper, "DELETE"):
		kw = "DELETE FROM "
	case strings.HasPrefix(upper, "MERGE"):
		kw = "MERGE "
	case strings.HasPrefix(upper, "UPDATE"):
		kw = "UPDATE "
	default:
		return nil
	}
	idx := strings.Index(upper, kw)
	if idx < 0 && kw == "MERGE " {
		// "MERGE INTO t": also accept without trailing space handling below.
	}
	p := idx + len(kw)
	if p > len(trimmed) {
		return nil
	}
	if kw == "MERGE " && strings.HasPrefix(upper[p:], "INTO ") {
		p += len("INTO ")
	}
	name := unquoteIdent(readIdent(trimmed[p:]))
	if name == "" || strings.HasPrefix(name, "(") {
		return nil
	}
	return []string{name}
}

// selectCacheTable reports the single table a plain SELECT reads from, used as
// the cache-key tag so writes to that table can invalidate precisely.
// It returns "" whenever caching would be unsound: non-SELECT statements,
// multiple relations (JOINs, comma lists), derived tables, or anything the
// lightweight scanner cannot attribute confidently.
func selectCacheTable(sql string) string {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return ""
	}
	head := strings.ToUpper(trimmed)
	if !strings.HasPrefix(head, "SELECT") && !strings.HasPrefix(head, "WITH") {
		return ""
	}
	upper := strings.ToUpper(sql)
	count := 0
	first := ""
	pos := 0
	for pos < len(upper) {
		fIdx := strings.Index(upper[pos:], " FROM ")
		jIdx := strings.Index(upper[pos:], " JOIN ")
		var idx int
		if fIdx >= 0 && (jIdx < 0 || fIdx < jIdx) {
			idx = pos + fIdx + len(" FROM ")
		} else if jIdx >= 0 {
			idx = pos + jIdx + len(" JOIN ")
		} else {
			break
		}
		tok := readIdent(sql[idx:])
		name := unquoteIdent(tok)
		if name == "" || strings.HasPrefix(name, "(") {
			return "" // derived table or unparseable — refuse to cache
		}
		count++
		if count == 1 {
			first = name
		}
		// Comma list: FROM a, b → more than one relation.
		rest := sql[idx+len(tok):]
		k := 0
		for k < len(rest) && isSpaceByte(rest[k]) {
			k++
		}
		if k < len(rest) && rest[k] == ',' {
			return ""
		}
		pos = idx + len(tok)
	}
	if count != 1 {
		return ""
	}
	return first
}

func readIdent(s string) string {
	i := 0
	for i < len(s) && isSpaceByte(s[i]) {
		i++
	}
	s = s[i:]
	if s == "" {
		return ""
	}
	switch s[0] {
	case '"', '`':
		q := s[0]
		j := 1
		for j < len(s) {
			if s[j] == q {
				if j+1 < len(s) && s[j+1] == q { // doubled quote escape
					j += 2
					continue
				}
				return s[:j+1]
			}
			j++
		}
		return s // unterminated; take the rest
	case '[':
		if j := strings.IndexByte(s, ']'); j > 0 {
			return s[:j+1]
		}
		return ""
	}
	j := 0
	for j < len(s) && isIdentChar(s[j]) {
		j++
	}
	return s[:j]
}

func unquoteIdent(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return s
	}
	switch {
	case s[0] == '"' && s[len(s)-1] == '"':
		return strings.ReplaceAll(s[1:len(s)-1], `""`, `"`)
	case s[0] == '`' && s[len(s)-1] == '`':
		return strings.ReplaceAll(s[1:len(s)-1], "``", "`")
	case s[0] == '[' && s[len(s)-1] == ']':
		return s[1 : len(s)-1]
	}
	return s
}

func isIdentChar(b byte) bool {
	return b == '_' || b == '$' || b == '#' || b == '.' ||
		(b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z')
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
