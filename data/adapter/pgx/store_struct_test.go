package pgx

import (
	"testing"
	"time"
)

func TestInsertColumns(t *testing.T) {
	type row struct {
		ID        int64     `db:"id"`
		Name      string    `db:"name"`
		Active    bool      `db:"active"`
		CreatedAt time.Time `db:"created_at"`
	}
	cols := insertColumns[row]("id")
	// id int (BIGSERIAL) omitted; timestamps included (ApplyEntityDefaults in CreateMany)
	if len(cols) != 3 || cols[0] != "name" || cols[1] != "active" || cols[2] != "created_at" {
		t.Fatalf("%v", cols)
	}
	vals := valuesForColumns(&row{Name: "K", Active: true}, cols)
	if vals[0] != "K" || vals[1] != true {
		t.Fatalf("%v", vals)
	}
}
