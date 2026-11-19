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

func TestInsertQueryOmitsReturning(t *testing.T) {
	type row struct {
		ID   string `db:"id"`
		Name string `db:"name"`
	}
	q, err := insertQuery(field.Entity{Name: "audit"}, &row{ID: "1", Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	ast := q.AST()
	if ast.Kind != expr.KindInsert || len(ast.Returning) != 0 {
		t.Fatalf("ast %+v", ast)
	}
	if _, err := insertQuery(field.Entity{Name: "audit"}, &struct{}{}); err == nil {
		t.Fatal("empty row accepted")
	}
	s := &Store[row]{table: "audit"}
	if _, err := s.Insert(context.Background(), nil); err == nil {
		t.Fatal("nil row accepted")
	}
}
