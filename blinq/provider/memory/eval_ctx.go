package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

type rowResolver func(table string) ([]map[string]any, error)

type evaluator struct {
	resolve rowResolver
	outer   map[string]any // parent row (correlated Exists/In)
}

func (e *evaluator) evalPred(ctx context.Context, p expr.Predicate, row map[string]any) (bool, error) {
	if p == nil {
		return true, nil
	}
	switch n := p.(type) {
	case expr.Binary:
		return e.evalBinary(ctx, n, row)
	case expr.And:
		for _, it := range n.Items {
			ok, err := e.evalPred(ctx, it, row)
			if err != nil || !ok {
				return ok, err
			}
		}
		return true, nil
	case expr.Or:
		for _, it := range n.Items {
			ok, err := e.evalPred(ctx, it, row)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	case expr.Not:
		ok, err := e.evalPred(ctx, n.Inner, row)
		return !ok, err
	case expr.Exists:
		return e.evalExists(ctx, n.Sub, row)
	case expr.RawPred:
		return false, fmt.Errorf("blinq/memory: RawPred unsupported")
	default:
		return false, fmt.Errorf("blinq/memory: unknown predicate %T", p)
	}
}

func (e *evaluator) evalExists(ctx context.Context, sub expr.Query, outerRow map[string]any) (bool, error) {
	prev := e.outer
	e.outer = outerRow
	defer func() { e.outer = prev }()
	rows, err := e.runSubquery(ctx, sub)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func (e *evaluator) evalBinary(ctx context.Context, b expr.Binary, row map[string]any) (bool, error) {
	if outer, ok := b.Value.(expr.OuterRef); ok {
		val, _ := lookupRow(row, b.Field.Column)
		ovar, ok2 := lookupRow(e.outer, outer.Field.Column)
		if !ok2 {
			return false, nil
		}
		switch b.Op {
		case expr.OpEq:
			return cmpEqual(val, ovar), nil
		case expr.OpNeq:
			return !cmpEqual(val, ovar), nil
		default:
			return false, fmt.Errorf("blinq/memory: OuterRef op %s", b.Op)
		}
	}
	if b.Op == expr.OpJSONContains || b.Op == expr.OpJSONPathEq {
		return evalJSON(b, row)
	}
	if b.Op == expr.OpIn {
		if sub, ok := asSubquery(b.Value); ok {
			return e.evalInSubquery(ctx, b.Field, row, sub)
		}
	}
	return evalBinary(b, row)
}

func (e *evaluator) evalInSubquery(ctx context.Context, field expr.FieldRef, row map[string]any, sub expr.Query) (bool, error) {
	val, ok := lookupRow(row, field.Column)
	if !ok {
		return false, nil
	}
	prev := e.outer
	e.outer = row
	defer func() { e.outer = prev }()
	rows, err := e.runSubquery(ctx, sub)
	if err != nil {
		return false, err
	}
	col := ""
	if len(sub.Selects) > 0 && !sub.Selects[0].All {
		col = sub.Selects[0].Field.Column
	}
	for _, r := range rows {
		var item any
		if col != "" {
			item, _ = lookupRow(r, col)
		} else if len(r) == 1 {
			for _, v := range r {
				item = v
			}
		} else {
			// uses the first stable column
			for _, v := range r {
				item = v
				break
			}
		}
		if cmpEqual(val, item) {
			return true, nil
		}
	}
	return false, nil
}

func (e *evaluator) runSubquery(ctx context.Context, sub expr.Query) ([]map[string]any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if e.resolve == nil {
		return nil, fmt.Errorf("blinq/memory: subquery requires multi-table provider")
	}
	src, err := e.resolve(sub.From.Name)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for _, row := range src {
		ok, err := e.evalPred(ctx, sub.Where, row)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, cloneRow(row))
		}
	}
	return applyOrderLimitProject(out, sub), nil
}

func asSubquery(v any) (expr.Query, bool) {
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

func lookupRow(row map[string]any, col string) (any, bool) {
	if row == nil {
		return nil, false
	}
	if v, ok := row[col]; ok {
		return v, true
	}
	for k, v := range row {
		if strings.EqualFold(k, col) {
			return v, true
		}
	}
	return nil, false
}

func evalJSON(b expr.Binary, row map[string]any) (bool, error) {
	colVal, ok := lookupRow(row, b.Field.Column)
	if !ok {
		return false, nil
	}
	jv, _ := b.Value.(expr.JSONValue)
	switch b.Op {
	case expr.OpJSONContains:
		want := jv.Value
		if want == nil {
			want = b.Value
		}
		// map/slice containment shallow
		switch cv := colVal.(type) {
		case map[string]any:
			wm, ok := want.(map[string]any)
			if !ok {
				return false, nil
			}
			for k, v := range wm {
				if !cmpEqual(cv[k], v) {
					return false, nil
				}
			}
			return true, nil
		default:
			return strings.Contains(fmt.Sprint(colVal), fmt.Sprint(want)), nil
		}
	case expr.OpJSONPathEq:
		path := jv.Path
		cur := colVal
		for _, part := range strings.Split(path, ".") {
			if part == "" {
				continue
			}
			m, ok := cur.(map[string]any)
			if !ok {
				return false, nil
			}
			cur, ok = m[part]
			if !ok {
				return false, nil
			}
		}
		return cmpEqual(cur, jv.Value), nil
	default:
		return false, fmt.Errorf("blinq/memory: json op %s", b.Op)
	}
}
