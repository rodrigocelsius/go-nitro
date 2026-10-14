package usecases

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestFilterAndSortItems(t *testing.T) {
	type Item struct {
		Title string `json:"title"`
		Done  bool   `json:"done"`
	}
	items := []any{
		&Item{Title: "a", Done: false},
		&Item{Title: "b", Done: true},
		&Item{Title: "c", Done: false},
	}

	c := &domain.Ctx{}
	// QueryParam needs AdapterData - use a simple mock
	c.AdapterData = queryMap{"done": "true"}

	filtered := filterItems(c, items, []string{"done", "title"})
	if len(filtered) != 1 {
		t.Fatalf("filtered = %d", len(filtered))
	}

	c2 := &domain.Ctx{AdapterData: queryMap{"sort": "-title"}}
	sorted := sortItems(c2, items, []string{"title"})
	if jsonFieldString(sorted[0], "title") != "c" {
		t.Fatalf("first = %v", sorted[0])
	}
}

type queryMap map[string]string

func (q queryMap) Query(key string) string { return q[key] }

func TestPaginateAnyItems(t *testing.T) {
	items := []any{"a", "b", "c", "d"}
	r := paginateAnyItems(items, 2, 2)
	if r.Total != 4 || len(r.Items) != 2 || r.Items[0] != "c" {
		t.Fatalf("%+v", r)
	}
}

func TestResolveResourceOpts_FilterableKeepsAuto(t *testing.T) {
	o := DefaultResourceOpts()
	o.Filterable = []string{"title"}
	got := resolveResourceOpts([]ResourceOpts{o})
	if !got.AutoID || !got.AutoTimestamps {
		t.Fatalf("%+v", got)
	}
	if !reflect.DeepEqual(got.Filterable, []string{"title"}) {
		t.Fatalf("%v", got.Filterable)
	}
}

func TestBuildListQuery_FilterRules(t *testing.T) {
	c := &domain.Ctx{AdapterData: queryMap{"min_price": "10", "name": "Ada"}}
	opts := ResourceOpts{
		Filterable:  []string{"name"},
		FilterRules: []FilterRule{{Param: "min_price", Field: "price", Op: "gte"}},
	}
	lq := buildListQuery(c, 1, 20, opts)
	if len(lq.Filters) != 2 {
		t.Fatalf("%+v", lq.Filters)
	}
	var foundGte bool
	for _, f := range lq.Filters {
		if f.Field == "price" && f.Op == "gte" && f.Value == "10" {
			foundGte = true
		}
	}
	if !foundGte {
		t.Fatalf("%+v", lq.Filters)
	}
}

func TestBuildListQuery_MaxFiltersAndValueLen(t *testing.T) {
	c := &domain.Ctx{AdapterData: queryMap{
		"a": "1", "b": "2", "c": "3",
		"long": strings.Repeat("x", 50),
	}}
	opts := ResourceOpts{
		Filterable:        []string{"a", "b", "c"},
		FilterRules:       []FilterRule{{Param: "long", Field: "long", Op: "eq"}},
		MaxFilters:        2,
		MaxFilterValueLen: 10,
	}
	lq := buildListQuery(c, 1, 20, opts)
	if len(lq.Filters) != 2 {
		t.Fatalf("want 2 filters, got %+v", lq.Filters)
	}
	for _, f := range lq.Filters {
		if f.Field == "long" && len(f.Value) != 10 {
			t.Fatalf("long value not truncated: %q", f.Value)
		}
	}
}

func TestClampListPagination(t *testing.T) {
	c := &domain.Ctx{AdapterData: queryMap{"page": "99999", "per_page": "5000"}}
	page, per := clampListPagination(c, ResourceOpts{MaxPage: 100, MaxPerPage: 50})
	if page != 100 || per != 50 {
		t.Fatalf("page=%d per=%d", page, per)
	}
	c2 := &domain.Ctx{AdapterData: queryMap{}}
	page, per = clampListPagination(c2, ResourceOpts{DefaultPerPage: 15})
	if page != 1 || per != 15 {
		t.Fatalf("defaults page=%d per=%d", page, per)
	}
}
