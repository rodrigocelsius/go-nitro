package sql

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
)

// Compiled is SQL + args.
type Compiled struct {
	SQL  string
	Args []any
}

// Compile translates AST → dialect SQL.
func Compile(q expr.Query, d dialect.Dialect) (Compiled, error) {
	switch q.Kind {
	case expr.KindSelect:
		return compileSelect(q, d)
	case expr.KindInsert:
		return compileInsert(q, d)
	case expr.KindUpsert:
		return compileUpsert(q, d)
	case expr.KindUpdate:
		return compileUpdate(q, d)
	case expr.KindDelete:
		return compileDelete(q, d)
	case expr.KindAggregate:
		return compileAggregate(q, d)
	default:
		return Compiled{}, fmt.Errorf("blinq/sql: unsupported kind %d", q.Kind)
	}
}

// CompileCount generates SELECT COUNT(*).
func CompileCount(q expr.Query, d dialect.Dialect) (Compiled, error) {
	aq := q
	aq.Kind = expr.KindAggregate
	aq.Aggregate = &expr.Aggregate{Func: expr.AggCount}
	aq.Selects = nil
	aq.OrderBy = nil
	aq.Limit = nil
	aq.Offset = nil
	aq.GroupBy = nil
	aq.Having = nil
	aq.Joins = q.Joins // COUNT with joins still counts joined rows
	return compileAggregate(aq, d)
}

func compileAggregate(q expr.Query, d dialect.Dialect) (Compiled, error) {
	if q.Aggregate == nil {
		return Compiled{}, fmt.Errorf("blinq/sql: aggregate missing")
	}
	agg := *q.Aggregate
	var sb strings.Builder
	var args []any
	idx := 1
	qualify := len(q.Joins) > 0

	sb.WriteString("SELECT ")
	sb.WriteString(renderAgg(agg, qualify, d))
	sb.WriteString(" FROM ")
	sb.WriteString(tableName(q.From, d))
	writeJoins(&sb, q.Joins, d)
	if q.Where != nil {
		w, a, next, err := compilePred(q.Where, d, idx, qualify)
		if err != nil {
			return Compiled{}, err
		}
		if w != "" {
			sb.WriteString(" WHERE ")
			sb.WriteString(w)
			args = append(args, a...)
			idx = next
		}
	}
	_ = idx
	return Compiled{SQL: sb.String(), Args: args}, nil
}

func renderAgg(agg expr.Aggregate, qualify bool, d dialect.Dialect) string {
	switch agg.Func {
	case expr.AggCount:
		if agg.Field.Column == "" {
			return "COUNT(*)"
		}
		return "COUNT(" + colName(agg.Field, qualify, d) + ")"
	default:
		return string(agg.Func) + "(" + colName(agg.Field, qualify, d) + ")"
	}
}

func tableName(e expr.EntityRef, d dialect.Dialect) string {
	name := d.QuoteIdent(e.Name)
	if e.Schema != "" {
		name = d.QuoteIdent(e.Schema) + "." + name
	}
	if e.Alias != "" {
		return name + " " + d.QuoteIdent(e.Alias)
	}
	return name
}

func colName(f expr.FieldRef, qualify bool, d dialect.Dialect) string {
	col := d.QuoteIdent(f.Column)
	if qualify && f.Table != "" {
		return d.QuoteIdent(f.Table) + "." + col
	}
	return col
}

func quoteOuter(f expr.FieldRef, d dialect.Dialect) string {
	if f.Table != "" {
		return d.QuoteIdent(f.Table) + "." + d.QuoteIdent(f.Column)
	}
	return d.QuoteIdent(f.Column)
}

func compileSelect(q expr.Query, d dialect.Dialect) (Compiled, error) {
	var sb strings.Builder
	var args []any
	idx := 1
	qualify := len(q.Joins) > 0

	sb.WriteString("SELECT ")
	if q.Distinct {
		sb.WriteString("DISTINCT ")
	}
	if top := d.SelectTop(q.Limit, q.Offset); top != "" {
		sb.WriteString(top)
	}
	if err := writeSelectList(&sb, q.Selects, qualify, d); err != nil {
		return Compiled{}, err
	}
	sb.WriteString(" FROM ")
	sb.WriteString(tableName(q.From, d))
	if q.Lock != nil && d.Name() == "mssql" {
		sb.WriteString(mssqlLockHint(q.Lock))
	}
	writeJoins(&sb, q.Joins, d)

	if q.Where != nil {
		w, a, next, err := compilePred(q.Where, d, idx, qualify)
		if err != nil {
			return Compiled{}, err
		}
		if w != "" {
			sb.WriteString(" WHERE ")
			sb.WriteString(w)
			args = append(args, a...)
			idx = next
		}
	}

	if q.Lock != nil && (len(q.GroupBy) > 0 || q.Distinct) {
		return Compiled{}, fmt.Errorf("blinq/sql: row locks cannot be combined with GROUP BY or DISTINCT")
	}

	if len(q.GroupBy) > 0 {
		sb.WriteString(" GROUP BY ")
		for i, f := range q.GroupBy {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(colName(f, qualify, d))
		}
	}

	if q.Having != nil {
		w, a, next, err := compilePred(q.Having, d, idx, qualify)
		if err != nil {
			return Compiled{}, err
		}
		if w != "" {
			sb.WriteString(" HAVING ")
			sb.WriteString(w)
			args = append(args, a...)
			idx = next
		}
	}

	orderBy := q.OrderBy
	lo := d.LimitOffset(q.Limit, q.Offset)
	// SQL Server OFFSET/FETCH requires ORDER BY; inject a stable no-op when missing.
	if lo != "" && len(orderBy) == 0 && d.Name() == "mssql" {
		sb.WriteString(" ORDER BY (SELECT NULL)")
	} else if len(orderBy) > 0 {
		sb.WriteString(" ORDER BY ")
		for i, o := range orderBy {
			if i > 0 {
				sb.WriteString(", ")
			}
			dir := o.Dir
			if dir != expr.OrderAsc && dir != expr.OrderDesc {
				return Compiled{}, fmt.Errorf("blinq/sql: Order.Dir must be ASC or DESC, got %q", dir)
			}
			sb.WriteString(colName(o.Field, qualify, d))
			sb.WriteString(" ")
			sb.WriteString(string(dir))
		}
	}

	sb.WriteString(lo)
	if q.Lock != nil {
		sb.WriteString(lockSuffix(q.Lock, d))
	}
	_ = idx
	return Compiled{SQL: sb.String(), Args: args}, nil
}

// lockSuffix renders FOR UPDATE / FOR SHARE [SKIP LOCKED | NOWAIT] for
// Postgres and MySQL 8+. SQLite locks the whole database on write and has no
// row locks (ignored); SQL Server uses table hints (see mssqlLockHint).
func lockSuffix(l *expr.LockClause, d dialect.Dialect) string {
	switch d.Name() {
	case "postgres", "mysql":
	default:
		return ""
	}
	out := " FOR UPDATE"
	if l.Share {
		out = " FOR SHARE"
	}
	if l.SkipLocked {
		out += " SKIP LOCKED"
	} else if l.NoWait {
		out += " NOWAIT"
	}
	return out
}

func mssqlLockHint(l *expr.LockClause) string {
	hints := []string{"ROWLOCK"}
	if l.Share {
		hints = append(hints, "HOLDLOCK")
	} else {
		hints = append(hints, "UPDLOCK")
	}
	if l.SkipLocked {
		hints = append(hints, "READPAST")
	} else if l.NoWait {
		hints = append(hints, "NOWAIT")
	}
	return " WITH (" + strings.Join(hints, ", ") + ")"
}

func writeSelectList(sb *strings.Builder, selects []expr.SelectItem, qualify bool, d dialect.Dialect) error {
	if len(selects) == 0 || selects[0].All {
		sb.WriteString("*")
		return nil
	}
	for i, s := range selects {
		if i > 0 {
			sb.WriteString(", ")
		}
		switch {
		case s.Agg != nil:
			sb.WriteString(renderAgg(*s.Agg, qualify, d))
		case s.All:
			sb.WriteString("*")
		default:
			sb.WriteString(colName(s.Field, qualify, d))
		}
		if s.Alias != "" {
			sb.WriteString(" AS ")
			sb.WriteString(d.QuoteIdent(s.Alias))
		}
	}
	return nil
}

func writeJoins(sb *strings.Builder, joins []expr.Join, d dialect.Dialect) {
	for _, j := range joins {
		sb.WriteString(" ")
		sb.WriteString(string(j.Type))
		sb.WriteString(" JOIN ")
		sb.WriteString(tableName(j.To, d))
		sb.WriteString(" ON ")
		sb.WriteString(colName(j.Left, true, d))
		sb.WriteString(" = ")
		sb.WriteString(colName(j.Right, true, d))
	}
}

func compilePred(p expr.Predicate, d dialect.Dialect, idx int, qualify bool) (string, []any, int, error) {
	switch n := p.(type) {
	case expr.Binary:
		return compileBinary(n, d, idx, qualify)
	case expr.And:
		return compileList(n.Items, " AND ", d, idx, qualify)
	case expr.Or:
		sql, args, next, err := compileList(n.Items, " OR ", d, idx, qualify)
		if err != nil {
			return "", nil, idx, err
		}
		return "(" + sql + ")", args, next, nil
	case expr.Not:
		inner, args, next, err := compilePred(n.Inner, d, idx, qualify)
		if err != nil {
			return "", nil, idx, err
		}
		return "NOT (" + inner + ")", args, next, nil
	case expr.Exists:
		inner, err := Compile(n.Sub, d)
		if err != nil {
			return "", nil, idx, err
		}
		sql, args, next := embedSubquery(inner, d, idx)
		return "EXISTS (" + sql + ")", args, next, nil
	case expr.RawPred:
		return "(" + n.SQL + ")", append([]any(nil), n.Args...), idx + len(n.Args), nil
	default:
		return "", nil, idx, fmt.Errorf("blinq/sql: unknown pred %T", p)
	}
}

func compileList(items []expr.Predicate, sep string, d dialect.Dialect, idx int, qualify bool) (string, []any, int, error) {
	var parts []string
	var args []any
	for _, it := range items {
		s, a, next, err := compilePred(it, d, idx, qualify)
		if err != nil {
			return "", nil, idx, err
		}
		if s == "" {
			continue
		}
		parts = append(parts, s)
		args = append(args, a...)
		idx = next
	}
	return strings.Join(parts, sep), args, idx, nil
}

func compileBinary(b expr.Binary, d dialect.Dialect, idx int, qualify bool) (string, []any, int, error) {
	col := colName(b.Field, qualify, d)
	switch b.Op {
	case expr.OpIsNull, expr.OpIsNotNull:
		return col + " " + string(b.Op), nil, idx, nil
	case expr.OpIn:
		if sub, ok := asSubqueryValue(b.Value); ok {
			inner, err := Compile(sub, d)
			if err != nil {
				return "", nil, idx, err
			}
			sql, args, next := embedSubquery(inner, d, idx)
			return col + " IN (" + sql + ")", args, next, nil
		}
		vals, ok := b.Value.([]any)
		if !ok {
			return "", nil, idx, fmt.Errorf("IN expects []any or subquery")
		}
		if len(vals) == 0 {
			return "1=0", nil, idx, nil
		}
		var sb strings.Builder
		var args []any
		sb.WriteString(col)
		sb.WriteString(" IN (")
		for i, v := range vals {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(d.Placeholder(idx))
			args = append(args, v)
			idx++
		}
		sb.WriteString(")")
		return sb.String(), args, idx, nil
	case expr.OpBetween:
		vals, ok := b.Value.([]any)
		if !ok || len(vals) != 2 {
			return "", nil, idx, fmt.Errorf("BETWEEN expects 2 values")
		}
		s := col + " BETWEEN " + d.Placeholder(idx) + " AND " + d.Placeholder(idx+1)
		return s, []any{vals[0], vals[1]}, idx + 2, nil
	case expr.OpLike:
		// ESCAPE '\' makes the backslash-escaping applied by
		// field.Contains/StartsWith/EndsWith (see field.go) effective
		// across dialects — without it, Postgres/MySQL already default
		// to backslash as the LIKE escape char, but SQLite/MSSQL do
		// not, so user-supplied "%"/"_" would still be interpreted as
		// wildcards there ("LIKE injection", see vuln.md V-34).
		s := col + " LIKE " + d.Placeholder(idx) + ` ESCAPE '\'`
		return s, []any{b.Value}, idx + 1, nil
	case expr.OpILike:
		s := d.RenderILike(col, d.Placeholder(idx)) + ` ESCAPE '\'`
		return s, []any{b.Value}, idx + 1, nil
	case expr.OpJSONContains, expr.OpJSONPathEq:
		return compileJSON(b, col, d, idx)
	default:
		if outer, ok := b.Value.(expr.OuterRef); ok {
			// Correlation: qualify both sides (table.col = outer.table.col).
			left := colName(b.Field, true, d)
			return left + " " + string(b.Op) + " " + quoteOuter(outer.Field, d), nil, idx, nil
		}
		s := col + " " + string(b.Op) + " " + d.Placeholder(idx)
		return s, []any{b.Value}, idx + 1, nil
	}
}

func compileJSON(b expr.Binary, col string, d dialect.Dialect, idx int) (string, []any, int, error) {
	jv, ok := b.Value.(expr.JSONValue)
	if !ok {
		jv = expr.JSONValue{Value: b.Value}
	}
	switch d.Name() {
	case "postgres":
		switch b.Op {
		case expr.OpJSONContains:
			arg, err := jsonArg(jv.Value)
			if err != nil {
				return "", nil, idx, err
			}
			return col + " @> " + d.Placeholder(idx) + "::jsonb", []any{arg}, idx + 1, nil
		case expr.OpJSONPathEq:
			segs, err := splitJSONPath(jv.Path)
			if err != nil {
				return "", nil, idx, err
			}
			// col #>> '{a,b}' = $n  (path segments by .)
			extract := col + " #>> '{" + postgresJSONPath(segs) + "}'"
			return extract + " = " + d.Placeholder(idx), []any{fmt.Sprint(jv.Value)}, idx + 1, nil
		}
	case "sqlite", "mysql":
		switch b.Op {
		case expr.OpJSONContains:
			// approximation: JSON_EXTRACT root LIKE '%value%' — a clear ErrUnsupported is preferable
			return "", nil, idx, fmt.Errorf("%w: JSONContains on %s (use postgres)", provider.ErrUnsupported, d.Name())
		case expr.OpJSONPathEq:
			segs, err := splitJSONPath(jv.Path)
			if err != nil {
				return "", nil, idx, err
			}
			ph := d.Placeholder(idx)
			fn := "JSON_EXTRACT"
			if d.Name() == "sqlite" {
				fn = "json_extract"
			}
			extract := fn + "(" + col + ", '" + mysqlJSONPath(segs) + "')"
			return extract + " = " + ph, []any{jv.Value}, idx + 1, nil
		}
	default:
		return "", nil, idx, fmt.Errorf("%w: JSON ops on %s", provider.ErrUnsupported, d.Name())
	}
	return "", nil, idx, fmt.Errorf("%w: JSON op %s", provider.ErrUnsupported, b.Op)
}

func jsonArg(v any) (any, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case []byte:
		return string(t), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return string(b), nil
	}
}

// splitJSONPath splits a dot-separated JSON path into non-empty segments.
// It rejects malformed paths and control/backslash characters so hostile
// input can never break out of the SQL literal rendered downstream
// (injection guard for OpJSONPathEq).
func splitJSONPath(path string) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("blinq/sql: JSONPathEq requires path")
	}
	segs := strings.Split(path, ".")
	for _, s := range segs {
		if s == "" {
			return nil, fmt.Errorf("blinq/sql: invalid JSON path %q", path)
		}
		for _, r := range s {
			if r < 0x20 || r == 0x7f || r == '\\' {
				return nil, fmt.Errorf("blinq/sql: invalid character %q in JSON path %q", r, path)
			}
		}
	}
	return segs, nil
}

// postgresJSONPath renders segments for the #>> operator: {a,b,c}.
// Single quotes are doubled so a segment cannot terminate the SQL literal.
func postgresJSONPath(segs []string) string {
	escaped := make([]string, len(segs))
	for i, s := range segs {
		escaped[i] = strings.ReplaceAll(s, "'", "''")
	}
	return strings.Join(escaped, ",")
}

// mysqlJSONPath renders a MySQL/SQLite JSON path ($.a.b). Segments with
// characters outside [A-Za-z0-9_-] are double-quoted ($."odd key"."x") with
// inner quotes doubled; single quotes are doubled for the outer SQL literal.
func mysqlJSONPath(segs []string) string {
	var b strings.Builder
	b.WriteByte('$')
	for _, s := range segs {
		if plainJSONSeg(s) {
			b.WriteString("." + s)
			continue
		}
		b.WriteString(`."` + strings.ReplaceAll(s, `"`, `""`) + `"`)
	}
	return strings.ReplaceAll(b.String(), "'", "''")
}

func plainJSONSeg(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

func asSubqueryValue(v any) (expr.Query, bool) {
	switch t := v.(type) {
	case expr.Subquery:
		return t.Query, true
	case *expr.Query:
		if t != nil {
			return *t, true
		}
	case expr.Query:
		return t, true
	}
	return expr.Query{}, false
}

// embedSubquery reindexes Postgres placeholders ($n) when the subquery is nested.
func embedSubquery(inner Compiled, d dialect.Dialect, idx int) (string, []any, int) {
	if d.Name() != "postgres" || len(inner.Args) == 0 || idx == 1 {
		return inner.SQL, inner.Args, idx + len(inner.Args)
	}
	sql := inner.SQL
	n := len(inner.Args)
	for i := n; i >= 1; i-- {
		old := d.Placeholder(i)
		tok := fmt.Sprintf("\x00ph%d\x00", i)
		sql = replacePlaceholder(sql, old, tok)
	}
	for i := 1; i <= n; i++ {
		tok := fmt.Sprintf("\x00ph%d\x00", i)
		sql = strings.ReplaceAll(sql, tok, d.Placeholder(idx+i-1))
	}
	return sql, inner.Args, idx + n
}

func replacePlaceholder(sql, old, neu string) string {
	// avoids $1 eating the prefix of $10: replace only if followed by a non-digit or end
	var b strings.Builder
	i := 0
	for i < len(sql) {
		if strings.HasPrefix(sql[i:], old) {
			end := i + len(old)
			if end == len(sql) || sql[end] < '0' || sql[end] > '9' {
				b.WriteString(neu)
				i = end
				continue
			}
		}
		b.WriteByte(sql[i])
		i++
	}
	return b.String()
}

func compileInsert(q expr.Query, d dialect.Dialect) (Compiled, error) {
	return compileInsertLike(q, d, false)
}

func compileUpsert(q expr.Query, d dialect.Dialect) (Compiled, error) {
	if d.Name() == "mssql" {
		return compileMergeMSSQL(q, d)
	}
	c, err := compileInsertLike(q, d, true)
	if err != nil {
		return Compiled{}, err
	}
	if q.Conflict == nil || len(q.Conflict.Columns) == 0 {
		return Compiled{}, fmt.Errorf("blinq/sql: upsert requires OnConflict columns")
	}
	suffix, err := d.UpsertClause(q)
	if err != nil {
		return Compiled{}, err
	}
	// RETURNING (if any) comes after the conflict clause on Postgres
	sql := c.SQL
	retIdx := strings.Index(strings.ToUpper(sql), " RETURNING ")
	if retIdx >= 0 {
		sql = sql[:retIdx] + suffix + sql[retIdx:]
	} else {
		sql += suffix
	}
	c.SQL = sql
	return c, nil
}

// compileMergeMSSQL generates a full MERGE (not INSERT+suffix).
func compileMergeMSSQL(q expr.Query, d dialect.Dialect) (Compiled, error) {
	if q.Conflict == nil || len(q.Conflict.Columns) == 0 {
		return Compiled{}, fmt.Errorf("blinq/sql: upsert requires OnConflict columns")
	}
	if len(q.Columns) == 0 {
		return Compiled{}, fmt.Errorf("blinq/sql: insert columns required")
	}
	rows := q.MultiValues
	if len(rows) == 0 {
		if len(q.Values) != len(q.Columns) {
			return Compiled{}, fmt.Errorf("blinq/sql: insert columns/values mismatch")
		}
		rows = [][]any{q.Values}
	}

	var sb strings.Builder
	var args []any
	idx := 1
	target := d.QuoteIdent("target")
	source := d.QuoteIdent("source")

	sb.WriteString("MERGE ")
	sb.WriteString(tableName(q.From, d))
	sb.WriteString(" AS ")
	sb.WriteString(target)
	sb.WriteString(" USING (VALUES ")
	for ri, row := range rows {
		if len(row) != len(q.Columns) {
			return Compiled{}, fmt.Errorf("blinq/sql: row %d length mismatch", ri)
		}
		if ri > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(")
		for ci := range row {
			if ci > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(d.Placeholder(idx))
			args = append(args, row[ci])
			idx++
		}
		sb.WriteString(")")
	}
	sb.WriteString(") AS ")
	sb.WriteString(source)
	sb.WriteString(" (")
	for i, c := range q.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(d.QuoteIdent(c))
	}
	sb.WriteString(") ON ")
	for i, col := range q.Conflict.Columns {
		if i > 0 {
			sb.WriteString(" AND ")
		}
		qc := d.QuoteIdent(col.Column)
		sb.WriteString(target)
		sb.WriteByte('.')
		sb.WriteString(qc)
		sb.WriteString(" = ")
		sb.WriteString(source)
		sb.WriteByte('.')
		sb.WriteString(qc)
	}

	switch q.Conflict.Action {
	case expr.ConflictDoNothing:
		// insert-only: omit WHEN MATCHED
	case expr.ConflictDoUpdate:
		sets, err := buildMSSQLMergeSets(d, q)
		if err != nil {
			return Compiled{}, err
		}
		sb.WriteString(" WHEN MATCHED THEN UPDATE SET ")
		sb.WriteString(sets)
	default:
		return Compiled{}, fmt.Errorf("blinq/sql: unknown conflict action")
	}

	sb.WriteString(" WHEN NOT MATCHED THEN INSERT (")
	for i, c := range q.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(d.QuoteIdent(c))
	}
	sb.WriteString(") VALUES (")
	for i, c := range q.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(dialect.ExcludedRef(d, c))
	}
	sb.WriteString(");")

	return Compiled{SQL: sb.String(), Args: args}, nil
}

func buildMSSQLMergeSets(d dialect.Dialect, q expr.Query) (string, error) {
	c := q.Conflict
	if c == nil {
		return "", fmt.Errorf("blinq/sql: upsert requires OnConflict columns")
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
			parts = append(parts, d.QuoteIdent(col)+" = "+dialect.ExcludedRef(d, col))
		}
		if len(parts) == 0 {
			return "", fmt.Errorf("blinq/sql: DoUpdate has no columns to update")
		}
		return strings.Join(parts, ", "), nil
	}
	parts := make([]string, len(sets))
	for i, s := range sets {
		col := s.Field.Column
		parts[i] = d.QuoteIdent(col) + " = " + dialect.ExcludedRef(d, col)
	}
	return strings.Join(parts, ", "), nil
}

func compileInsertLike(q expr.Query, d dialect.Dialect, _ bool) (Compiled, error) {
	if len(q.Columns) == 0 {
		return Compiled{}, fmt.Errorf("blinq/sql: insert columns required")
	}
	rows := q.MultiValues
	if len(rows) == 0 {
		if len(q.Values) != len(q.Columns) {
			return Compiled{}, fmt.Errorf("blinq/sql: insert columns/values mismatch")
		}
		rows = [][]any{q.Values}
	}

	var sb strings.Builder
	var args []any
	idx := 1
	sb.WriteString("INSERT INTO ")
	sb.WriteString(tableName(q.From, d))
	sb.WriteString(" (")
	for i, c := range q.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(d.QuoteIdent(c))
	}
	sb.WriteString(") VALUES ")
	for ri, row := range rows {
		if len(row) != len(q.Columns) {
			return Compiled{}, fmt.Errorf("blinq/sql: row %d length mismatch", ri)
		}
		if ri > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(")
		for ci := range row {
			if ci > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(d.Placeholder(idx))
			args = append(args, row[ci])
			idx++
		}
		sb.WriteString(")")
	}
	writeReturning(&sb, q, d)
	return Compiled{SQL: sb.String(), Args: args}, nil
}

func compileUpdate(q expr.Query, d dialect.Dialect) (Compiled, error) {
	if len(q.Sets) == 0 {
		return Compiled{}, fmt.Errorf("blinq/sql: update requires SET")
	}
	var sb strings.Builder
	var args []any
	idx := 1
	sb.WriteString("UPDATE ")
	sb.WriteString(tableName(q.From, d))
	sb.WriteString(" SET ")
	for i, s := range q.Sets {
		if i > 0 {
			sb.WriteString(", ")
		}
		col := colName(s.Field, false, d)
		sb.WriteString(col)
		sb.WriteString(" = ")
		switch v := s.Value.(type) {
		case expr.SoftNow:
			sb.WriteString(d.SoftDeleteNow())
		case expr.Inc:
			sb.WriteString(col)
			sb.WriteString(" + ")
			sb.WriteString(d.Placeholder(idx))
			args = append(args, v.Delta)
			idx++
		case expr.Expr:
			sb.WriteString(v.SQL)
			args = append(args, v.Args...)
			idx += len(v.Args)
		default:
			sb.WriteString(d.Placeholder(idx))
			args = append(args, s.Value)
			idx++
		}
	}
	if q.Where != nil {
		w, a, next, err := compilePred(q.Where, d, idx, false)
		if err != nil {
			return Compiled{}, err
		}
		if w != "" {
			sb.WriteString(" WHERE ")
			sb.WriteString(w)
			args = append(args, a...)
			idx = next
		}
	}
	_ = idx
	writeReturning(&sb, q, d)
	return Compiled{SQL: sb.String(), Args: args}, nil
}

func compileDelete(q expr.Query, d dialect.Dialect) (Compiled, error) {
	var sb strings.Builder
	var args []any
	idx := 1
	sb.WriteString("DELETE FROM ")
	sb.WriteString(tableName(q.From, d))
	if q.Where != nil {
		w, a, next, err := compilePred(q.Where, d, idx, false)
		if err != nil {
			return Compiled{}, err
		}
		if w != "" {
			sb.WriteString(" WHERE ")
			sb.WriteString(w)
			args = append(args, a...)
			idx = next
		}
	}
	_ = idx
	return Compiled{SQL: sb.String(), Args: args}, nil
}

func writeReturning(sb *strings.Builder, q expr.Query, d dialect.Dialect) {
	if !d.ReturningSupported() || q.NoReturning {
		return
	}
	for _, s := range q.Sets {
		if _, ok := s.Value.(expr.SoftNow); ok && len(q.Returning) == 0 {
			return
		}
	}
	sb.WriteString(" RETURNING ")
	if len(q.Returning) == 0 {
		sb.WriteString("*")
		return
	}
	for i, f := range q.Returning {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(d.QuoteIdent(f.Column))
	}
}
