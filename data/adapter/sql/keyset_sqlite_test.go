package sqladapter

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type keysetRow struct {
	ID   string `db:"id" json:"id"`
	Kind string `db:"kind" json:"kind"`
	Size int64  `db:"size" json:"size"`
}

// Keyset paging returns exactly the rows of the offset listing, in the same
// order, with no row repeated or skipped — including across duplicates of
// the sort column and with mixed directions.
func TestListQuery_KeysetMatchesOffsetOrder(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, "sqlite", "file:keysetdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.StdDB().ExecContext(ctx, `CREATE TABLE keyset_rows (id TEXT PRIMARY KEY, kind TEXT NOT NULL, size INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 23; i++ {
		if _, err := db.StdDB().ExecContext(ctx, `INSERT INTO keyset_rows VALUES (?, ?, ?)`,
			fmt.Sprintf("id-%02d", i), fmt.Sprintf("k%d", i%3), int64(i%5)); err != nil {
			t.Fatal(err)
		}
	}
	repo := StoreOf[keysetRow](db, "keyset_rows").AsResourceRepository()
	ql, ok := repo.(domain.QueryableLister)
	if !ok {
		t.Fatal("store repository is not a QueryableLister")
	}

	ids := func(items []any) []string {
		out := make([]string, len(items))
		for i, it := range items {
			out[i] = it.(*keysetRow).ID
		}
		return out
	}
	value := func(r *keysetRow, field string) string {
		switch field {
		case "kind":
			return r.Kind
		case "size":
			return fmt.Sprint(r.Size)
		}
		return r.ID
	}

	for name, sorts := range map[string][]domain.ListSort{
		"no sort":         nil,
		"kind asc":        {{Field: "kind"}},
		"kind desc":       {{Field: "kind", Desc: true}},
		"kind, size desc": {{Field: "kind"}, {Field: "size", Desc: true}},
		"id desc":         {{Field: "id", Desc: true}},
	} {
		all, err := ql.ListQuery(ctx, domain.ListQuery{Page: 1, PerPage: 100, Sort: sorts})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := ids(all.Items)
		if len(want) != 23 || all.Page != 1 {
			t.Fatalf("%s: offset listing has %d rows, page %d", name, len(want), all.Page)
		}

		var got []string
		var after *domain.ListCursor
		for pages := 0; pages < 10; pages++ {
			res, err := ql.ListQuery(ctx, domain.ListQuery{Page: 1, PerPage: 5, Sort: sorts, Keyset: true, After: after})
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if res.Page != 0 || res.Total != 0 {
				t.Fatalf("%s: keyset result must report page 0 and no total, got page %d total %d", name, res.Page, res.Total)
			}
			if len(res.Items) == 0 {
				break
			}
			got = append(got, ids(res.Items)...)
			last := res.Items[len(res.Items)-1].(*keysetRow)
			after = &domain.ListCursor{ID: last.ID}
			for _, s := range sorts {
				after.Values = append(after.Values, value(last, s.Field))
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\nkeyset = %v\noffset = %v", name, got, want)
		}
	}

	// Filters still apply next to the cursor.
	res, err := ql.ListQuery(ctx, domain.ListQuery{
		PerPage: 50, Keyset: true,
		Filters: []domain.ListFilter{{Field: "kind", Value: "k1", Op: "eq"}},
		After:   &domain.ListCursor{ID: "id-10"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range res.Items {
		if r := it.(*keysetRow); r.Kind != "k1" || r.ID <= "id-10" {
			t.Errorf("row %+v escapes the filter or the cursor", r)
		}
	}
	if len(res.Items) != 4 { // id-13, 16, 19, 22
		t.Errorf("filtered keyset page has %d rows, want 4", len(res.Items))
	}

	// A cursor that does not match the sort is rejected, not misapplied.
	if _, err := ql.ListQuery(ctx, domain.ListQuery{
		PerPage: 5, Keyset: true, Sort: []domain.ListSort{{Field: "kind"}},
		After: &domain.ListCursor{ID: "id-10"},
	}); err == nil {
		t.Error("cursor without the sort values must be an error")
	}
}
