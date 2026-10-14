package query

import (
	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
)

// Join adds INNER JOIN.
func (q Query) Join(to field.Entity, left, right expr.FieldRef) Query {
	return q.addJoin(expr.JoinInner, to, left, right)
}

// LeftJoin adds LEFT JOIN.
func (q Query) LeftJoin(to field.Entity, left, right expr.FieldRef) Query {
	return q.addJoin(expr.JoinLeft, to, left, right)
}

func (q Query) addJoin(typ expr.JoinType, to field.Entity, left, right expr.FieldRef) Query {
	q = q.clone()
	q.ast.Joins = append(append([]expr.Join(nil), q.ast.Joins...), expr.Join{
		Type:  typ,
		To:    to.Ref(),
		Left:  left,
		Right: right,
	})
	return q
}

// GroupBy sets GROUP BY.
func (q Query) GroupBy(fields ...expr.FieldRef) Query {
	q = q.clone()
	q.ast.GroupBy = append([]expr.FieldRef(nil), fields...)
	return q
}

// Having adds HAVING (after GroupBy).
func (q Query) Having(p field.Pred) Query {
	q = q.clone()
	node := p.Node()
	if q.ast.Having == nil {
		q.ast.Having = node
		return q
	}
	q.ast.Having = expr.And{Items: []expr.Predicate{q.ast.Having, node}}
	return q
}

// SelectItems sets the SELECT list (fields and/or aggregates).
func (q Query) SelectItems(items ...expr.SelectItem) Query {
	q = q.clone()
	q.ast.Selects = append([]expr.SelectItem(nil), items...)
	return q
}

// AggSelect creates an aggregate SelectItem (e.g. COUNT(*) AS n).
func AggSelect(agg expr.Aggregate, alias string) expr.SelectItem {
	return expr.SelectItem{Agg: &agg, Alias: alias}
}

// FieldSelect creates a column SelectItem with an optional alias.
func FieldSelect(f expr.FieldRef, alias string) expr.SelectItem {
	return expr.SelectItem{Field: f, Alias: alias}
}
