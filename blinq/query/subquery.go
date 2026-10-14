package query

import (
	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
)

// Exists creates an EXISTS predicate (SELECT subquery).
func Exists(sub Query) field.Pred {
	ast := sub.AST()
	ast.Includes = nil
	return field.PredFrom(expr.Exists{Sub: ast})
}

// InQuery is Field.In with a subquery: col IN (SELECT …).
func InQuery[T any](f field.Field[T], sub Query) field.Pred {
	ast := sub.AST()
	ast.Includes = nil
	return field.PredFrom(expr.Binary{
		Field: f.Ref(),
		Op:    expr.OpIn,
		Value: expr.Subquery{Query: ast},
	})
}
