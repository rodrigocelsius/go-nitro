package sql

import (
	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

// IndexSuggestion suggests an index from Where / OrderBy / Join keys.
type IndexSuggestion struct {
	Table   string
	Columns []string
	Reason  string // where|order|join
}

// SuggestIndexes analyzes the query and suggests indexes (simple heuristic).
func SuggestIndexes(q expr.Query) []IndexSuggestion {
	var out []IndexSuggestion
	seen := map[string]struct{}{}

	add := func(table string, cols []string, reason string) {
		if len(cols) == 0 {
			return
		}
		key := table + "|" + reason + "|" + joinCols(cols)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, IndexSuggestion{Table: table, Columns: cols, Reason: reason})
	}

	table := q.From.Name
	expr.WalkPred(q.Where, func(p expr.Predicate) bool {
		if b, ok := p.(expr.Binary); ok {
			t := b.Field.Table
			if t == "" {
				t = table
			}
			add(t, []string{b.Field.Column}, "where")
		}
		return true
	})

	for _, o := range q.OrderBy {
		t := o.Field.Table
		if t == "" {
			t = table
		}
		add(t, []string{o.Field.Column}, "order")
	}

	for _, j := range q.Joins {
		lt := j.Left.Table
		if lt == "" {
			lt = table
		}
		add(lt, []string{j.Left.Column}, "join")
		rt := j.Right.Table
		if rt == "" {
			rt = j.To.Name
		}
		add(rt, []string{j.Right.Column}, "join")
	}

	return out
}

func joinCols(cols []string) string {
	if len(cols) == 0 {
		return ""
	}
	s := cols[0]
	for i := 1; i < len(cols); i++ {
		s += "," + cols[i]
	}
	return s
}
