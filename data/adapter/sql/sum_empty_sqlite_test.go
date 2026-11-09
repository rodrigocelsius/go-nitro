package sqladapter

import (
	"context"
	"testing"

	blinqfield "github.com/rodrigocelsius/go-nitro/blinq/v2/field"
)

type sumRow struct {
	ID   string `db:"id"`
	Kind string `db:"kind"`
	Size int64  `db:"size"`
}

// SUM over no rows is NULL in SQL; the aggregate helpers report 0.
func TestSumOverNoRowsIsZero(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, "sqlite", "file:sumemptydb?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.StdDB().ExecContext(ctx, `CREATE TABLE sum_rows (id TEXT PRIMARY KEY, kind TEXT, size INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.StdDB().ExecContext(ctx, `INSERT INTO sum_rows VALUES ('1', 'a', 5), ('2', 'a', 7)`); err != nil {
		t.Fatal(err)
	}
	store := StoreOf[sumRow](db, "sum_rows")
	kind := blinqfield.Col[string]("sum_rows", "kind")
	size := blinqfield.Col[int64]("sum_rows", "size")

	got, err := store.Query().Where(kind.Eq("a")).SumInt64(ctx, store.API().Provider(), size.Ref())
	if err != nil || got != 12 {
		t.Fatalf("sum of a = %d, %v; want 12", got, err)
	}
	got, err = store.Query().Where(kind.Eq("none")).SumInt64(ctx, store.API().Provider(), size.Ref())
	if err != nil || got != 0 {
		t.Fatalf("sum of nothing = %d, %v; want 0, nil", got, err)
	}
	max, err := store.Query().Where(kind.Eq("none")).Max(ctx, store.API().Provider(), size.Ref())
	if err != nil || max != 0 {
		t.Fatalf("max of nothing = %v, %v", max, err)
	}
}
