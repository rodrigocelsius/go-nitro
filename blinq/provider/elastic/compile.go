// Package elastic compiles a BLinq AST subset to Elasticsearch Query DSL (map),
// without depending on the official client — the app sends the body with its client.
package elastic

import (
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

// Search is the equivalent Search API body (subset).
type Search struct {
	Index string
	Body  map[string]any
}

// CompileSearch translates a SELECT subset → Query DSL (bool.filter, sort, from/size).
func CompileSearch(q expr.Query) (Search, error) {
	if q.Kind != expr.KindSelect && q.Kind != expr.KindAggregate {
		return Search{}, fmt.Errorf("blinq/elastic: only Select/Aggregate (got %d)", q.Kind)
	}
	if len(q.Joins) > 0 || len(q.GroupBy) > 0 || q.Having != nil {
		return Search{}, fmt.Errorf("blinq/elastic: Join/GroupBy/Having not supported in this subset")
	}
	if len(q.Includes) > 0 {
		return Search{}, fmt.Errorf("blinq/elastic: Include not supported in this subset")
	}

	body := map[string]any{}
	if q.Where != nil {
		filter, err := compilePred(q.Where)
		if err != nil {
			return Search{}, err
		}
		body["query"] = map[string]any{
			"bool": map[string]any{
				"filter": filter,
			},
		}
	} else {
		body["query"] = map[string]any{"match_all": map[string]any{}}
	}

	if len(q.OrderBy) > 0 {
		sort := make([]map[string]any, 0, len(q.OrderBy))
		for _, o := range q.OrderBy {
			dir := "asc"
			if o.Dir == expr.OrderDesc {
				dir = "desc"
			}
			sort = append(sort, map[string]any{
				o.Field.Column: map[string]any{"order": dir},
			})
		}
		body["sort"] = sort
	}
	if q.Offset != nil {
		body["from"] = int(*q.Offset)
	}
	if q.Limit != nil {
		body["size"] = int(*q.Limit)
	}

	return Search{Index: q.From.Name, Body: body}, nil
}

func compilePred(p expr.Predicate) (any, error) {
	switch n := p.(type) {
	case expr.Binary:
		return compileBinary(n)
	case expr.And:
		parts := make([]any, 0, len(n.Items))
		for _, it := range n.Items {
			m, err := compilePred(it)
			if err != nil {
				return nil, err
			}
			parts = append(parts, m)
		}
		return map[string]any{"bool": map[string]any{"filter": parts}}, nil
	case expr.Or:
		parts := make([]any, 0, len(n.Items))
		for _, it := range n.Items {
			m, err := compilePred(it)
			if err != nil {
				return nil, err
			}
			parts = append(parts, m)
		}
		return map[string]any{"bool": map[string]any{"should": parts, "minimum_should_match": 1}}, nil
	case expr.Not:
		inner, err := compilePred(n.Inner)
		if err != nil {
			return nil, err
		}
		return map[string]any{"bool": map[string]any{"must_not": []any{inner}}}, nil
	case expr.Exists:
		return nil, fmt.Errorf("blinq/elastic: Exists subquery not supported in this subset")
	default:
		return nil, fmt.Errorf("blinq/elastic: pred %T", p)
	}
}

func compileBinary(b expr.Binary) (map[string]any, error) {
	col := b.Field.Column
	switch b.Op {
	case expr.OpEq:
		return map[string]any{"term": map[string]any{col: b.Value}}, nil
	case expr.OpNeq:
		return map[string]any{"bool": map[string]any{
			"must_not": []any{map[string]any{"term": map[string]any{col: b.Value}}},
		}}, nil
	case expr.OpGt:
		return map[string]any{"range": map[string]any{col: map[string]any{"gt": b.Value}}}, nil
	case expr.OpGte:
		return map[string]any{"range": map[string]any{col: map[string]any{"gte": b.Value}}}, nil
	case expr.OpLt:
		return map[string]any{"range": map[string]any{col: map[string]any{"lt": b.Value}}}, nil
	case expr.OpLte:
		return map[string]any{"range": map[string]any{col: map[string]any{"lte": b.Value}}}, nil
	case expr.OpIn:
		vals, ok := b.Value.([]any)
		if !ok {
			return nil, fmt.Errorf("blinq/elastic: IN subquery not supported")
		}
		return map[string]any{"terms": map[string]any{col: vals}}, nil
	case expr.OpBetween:
		vals, ok := b.Value.([]any)
		if !ok || len(vals) != 2 {
			return nil, fmt.Errorf("BETWEEN expects 2 values")
		}
		return map[string]any{"range": map[string]any{col: map[string]any{"gte": vals[0], "lte": vals[1]}}}, nil
	case expr.OpIsNull:
		return map[string]any{"bool": map[string]any{
			"must_not": []any{map[string]any{"exists": map[string]any{"field": col}}},
		}}, nil
	case expr.OpIsNotNull:
		return map[string]any{"exists": map[string]any{"field": col}}, nil
	case expr.OpLike, expr.OpILike:
		pat, _ := b.Value.(string)
		return map[string]any{"wildcard": map[string]any{col: likeToWildcard(pat)}}, nil
	default:
		return nil, fmt.Errorf("blinq/elastic: op %s", b.Op)
	}
}

// likeToWildcard converts a LIKE pattern to Elasticsearch's "wildcard"
// query syntax (% → *, _ → ?). A backslash immediately before '%', '_'
// or '\' escapes it back to a literal character (matching
// field.Contains/StartsWith/EndsWith's backslash-escaping and the SQL
// providers' "ESCAPE '\'" — see vuln.md V-34); since '\' is itself the
// escape character in Elasticsearch's wildcard syntax, a literal
// backslash is re-escaped as "\\" in the output.
func likeToWildcard(pat string) string {
	var b []byte
	for i := 0; i < len(pat); i++ {
		c := pat[i]
		if c == '\\' && i+1 < len(pat) {
			i++
			if pat[i] == '\\' {
				b = append(b, '\\', '\\')
			} else {
				b = append(b, pat[i])
			}
			continue
		}
		switch c {
		case '%':
			b = append(b, '*')
		case '_':
			b = append(b, '?')
		default:
			b = append(b, c)
		}
	}
	return string(b)
}
