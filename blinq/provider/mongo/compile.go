// Package mongo compiles a BLinq AST subset to MongoDB-style filters (map),
// without depending on the official driver — the app applies the filter with its client.
package mongo

import (
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

// Find is the equivalent Find plan.
type Find struct {
	Collection string
	Filter     map[string]any
	Sort       map[string]int // 1 asc, -1 desc
	Limit      *int64
	Skip       *int64
}

// CompileFind translates a SELECT subset → Find (Where/Order/Limit/Offset).
func CompileFind(q expr.Query) (Find, error) {
	if q.Kind != expr.KindSelect && q.Kind != expr.KindAggregate {
		return Find{}, fmt.Errorf("blinq/mongo: only Select/Aggregate (got %d)", q.Kind)
	}
	if len(q.Joins) > 0 || len(q.GroupBy) > 0 || q.Having != nil {
		return Find{}, fmt.Errorf("blinq/mongo: Join/GroupBy/Having not supported in this subset")
	}
	if len(q.Includes) > 0 {
		return Find{}, fmt.Errorf("blinq/mongo: Include uses client-side batching; it is not part of the filter")
	}
	f := Find{Collection: q.From.Name}
	if q.Where != nil {
		filter, err := compilePred(q.Where)
		if err != nil {
			return Find{}, err
		}
		f.Filter = filter
	} else {
		f.Filter = map[string]any{}
	}
	if len(q.OrderBy) > 0 {
		f.Sort = make(map[string]int, len(q.OrderBy))
		for _, o := range q.OrderBy {
			dir := 1
			if o.Dir == expr.OrderDesc {
				dir = -1
			}
			f.Sort[o.Field.Column] = dir
		}
	}
	if q.Limit != nil {
		n := int64(*q.Limit)
		f.Limit = &n
	}
	if q.Offset != nil {
		n := int64(*q.Offset)
		f.Skip = &n
	}
	return f, nil
}

// CompileLookup translates Include → $lookup stage (aggregation pipeline).
func CompileLookup(inc expr.Include) map[string]any {
	local := inc.LocalKey.Column
	foreign := inc.ForeignKey.Column
	as := inc.Name
	if as == "" {
		as = inc.Target.Name
	}
	from := inc.Target.Name
	return map[string]any{
		"$lookup": map[string]any{
			"from":         from,
			"localField":   local,
			"foreignField": foreign,
			"as":           as,
		},
	}
}

func compilePred(p expr.Predicate) (map[string]any, error) {
	switch n := p.(type) {
	case expr.Binary:
		return compileBinary(n)
	case expr.And:
		parts := make([]map[string]any, 0, len(n.Items))
		for _, it := range n.Items {
			m, err := compilePred(it)
			if err != nil {
				return nil, err
			}
			parts = append(parts, m)
		}
		return map[string]any{"$and": parts}, nil
	case expr.Or:
		parts := make([]map[string]any, 0, len(n.Items))
		for _, it := range n.Items {
			m, err := compilePred(it)
			if err != nil {
				return nil, err
			}
			parts = append(parts, m)
		}
		return map[string]any{"$or": parts}, nil
	case expr.Not:
		inner, err := compilePred(n.Inner)
		if err != nil {
			return nil, err
		}
		return map[string]any{"$nor": []map[string]any{inner}}, nil
	case expr.Exists:
		return nil, fmt.Errorf("blinq/mongo: Exists subquery not supported in this subset")
	default:
		return nil, fmt.Errorf("blinq/mongo: pred %T", p)
	}
}

func compileBinary(b expr.Binary) (map[string]any, error) {
	col := b.Field.Column
	switch b.Op {
	case expr.OpEq:
		return map[string]any{col: b.Value}, nil
	case expr.OpNeq:
		return map[string]any{col: map[string]any{"$ne": b.Value}}, nil
	case expr.OpGt:
		return map[string]any{col: map[string]any{"$gt": b.Value}}, nil
	case expr.OpGte:
		return map[string]any{col: map[string]any{"$gte": b.Value}}, nil
	case expr.OpLt:
		return map[string]any{col: map[string]any{"$lt": b.Value}}, nil
	case expr.OpLte:
		return map[string]any{col: map[string]any{"$lte": b.Value}}, nil
	case expr.OpIn:
		if _, ok := asSub(b.Value); ok {
			return nil, fmt.Errorf("blinq/mongo: IN subquery not supported")
		}
		vals, _ := b.Value.([]any)
		return map[string]any{col: map[string]any{"$in": vals}}, nil
	case expr.OpBetween:
		vals, ok := b.Value.([]any)
		if !ok || len(vals) != 2 {
			return nil, fmt.Errorf("BETWEEN expects 2 values")
		}
		return map[string]any{col: map[string]any{"$gte": vals[0], "$lte": vals[1]}}, nil
	case expr.OpIsNull:
		return map[string]any{col: nil}, nil
	case expr.OpIsNotNull:
		return map[string]any{col: map[string]any{"$ne": nil}}, nil
	case expr.OpLike, expr.OpILike:
		pat, _ := b.Value.(string)
		return map[string]any{col: map[string]any{"$regex": likeToRegex(pat), "$options": "i"}}, nil
	default:
		return nil, fmt.Errorf("blinq/mongo: op %s", b.Op)
	}
}

func asSub(v any) (expr.Query, bool) {
	switch t := v.(type) {
	case expr.Subquery:
		return t.Query, true
	case expr.Query:
		return t, true
	case *expr.Query:
		if t != nil {
			return *t, true
		}
	}
	return expr.Query{}, false
}

// escapeRegexLiteral appends c to b as a regex-safe literal.
func escapeRegexLiteral(b []byte, c byte) []byte {
	switch c {
	case '.', '*', '+', '?', '(', ')', '[', ']', '{', '}', '\\', '|', '^', '$':
		return append(b, '\\', c)
	default:
		return append(b, c)
	}
}

func likeToRegex(pat string) string {
	// % → .*, _ → ., escape the rest. A backslash immediately before
	// '%', '_' or '\' escapes it back to a literal character instead of
	// a wildcard (matching field.Contains/StartsWith/EndsWith's
	// backslash-escaping and the SQL providers' "ESCAPE '\'" — see
	// vuln.md V-34), so "50\%" matches the literal text "50%" instead of
	// "50" followed by anything.
	var b []byte
	b = append(b, '^')
	for i := 0; i < len(pat); i++ {
		c := pat[i]
		if c == '\\' && i+1 < len(pat) {
			i++
			b = escapeRegexLiteral(b, pat[i])
			continue
		}
		switch c {
		case '%':
			b = append(b, '.', '*')
		case '_':
			b = append(b, '.')
		default:
			b = escapeRegexLiteral(b, c)
		}
	}
	b = append(b, '$')
	return string(b)
}
