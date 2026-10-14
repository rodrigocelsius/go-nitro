package pgx

import (
	"reflect"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

func idFromAST(q expr.Query, idCol string) any {
	switch q.Kind {
	case expr.KindInsert, expr.KindUpsert:
		for i, c := range q.Columns {
			if c == idCol && i < len(q.Values) {
				return q.Values[i]
			}
		}
	case expr.KindUpdate:
		return idFromPred(q.Where, idCol)
	}
	return nil
}

func idFromPred(p expr.Predicate, idCol string) any {
	if p == nil {
		return nil
	}
	switch t := p.(type) {
	case expr.Binary:
		if t.Field.Column == idCol && t.Op == expr.OpEq {
			return t.Value
		}
	case *expr.Binary:
		if t != nil && t.Field.Column == idCol && t.Op == expr.OpEq {
			return t.Value
		}
	case expr.And:
		for _, item := range t.Items {
			if v := idFromPred(item, idCol); v != nil {
				return v
			}
		}
	case *expr.And:
		if t == nil {
			return nil
		}
		for _, item := range t.Items {
			if v := idFromPred(item, idCol); v != nil {
				return v
			}
		}
	}
	return nil
}

func isEmptyID(id any) bool {
	if id == nil {
		return true
	}
	v := reflect.ValueOf(id)
	if !v.IsValid() {
		return true
	}
	return v.IsZero()
}
