package pgx

import (
	"context"
	"errors"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestDeleteWhereRefusesUnfilteredDeletes(t *testing.T) {
	e := field.Entity{Name: "sessions"}
	s := &Store[hardModel]{table: "sessions"}
	if _, err := s.DeleteWhere(context.Background(), query.From(e)); !errors.Is(err, ErrUnfilteredDelete) {
		t.Fatalf("unfiltered delete: %v", err)
	}
	q, err := deleteWhere(e, query.From(e).Where(field.Col[string]("sessions", "id").Eq("x")))
	if err != nil {
		t.Fatal(err)
	}
	ast := q.AST()
	if ast.Kind != expr.KindDelete || ast.Where == nil {
		t.Fatalf("ast %+v", ast)
	}
}
