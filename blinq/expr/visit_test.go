package expr_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
)

func TestWalkPred(t *testing.T) {
	p := expr.And{Items: []expr.Predicate{
		expr.Binary{Field: expr.FieldRef{Column: "a"}, Op: expr.OpEq, Value: 1},
		expr.Not{Inner: expr.Binary{Field: expr.FieldRef{Column: "b"}, Op: expr.OpIsNull}},
	}}
	var n int
	expr.WalkPred(p, func(pred expr.Predicate) bool {
		n++
		return true
	})
	if n < 3 {
		t.Fatalf("visited %d", n)
	}
}
