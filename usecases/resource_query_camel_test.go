package usecases

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestBuildListQuery_CamelCaseAlias(t *testing.T) {
	c := &domain.Ctx{AdapterData: queryMap{"artistId": "uuid-1"}}
	opts := ResourceOpts{
		FilterRules: []FilterRule{{Param: "artist_id", Field: "artist_id", Op: "eq"}},
	}
	lq := buildListQuery(c, 1, 20, opts)
	if len(lq.Filters) != 1 {
		t.Fatalf("filters=%+v", lq.Filters)
	}
	if lq.Filters[0].Field != "artist_id" || lq.Filters[0].Value != "uuid-1" {
		t.Fatalf("%+v", lq.Filters[0])
	}
}

func TestBuildListQuery_SortCamelCase(t *testing.T) {
	c := &domain.Ctx{AdapterData: queryMap{"sort": "-createdAt"}}
	opts := ResourceOpts{Sortable: []string{"created_at"}}
	lq := buildListQuery(c, 1, 20, opts)
	if len(lq.Sort) != 1 || lq.Sort[0].Field != "created_at" || !lq.Sort[0].Desc {
		t.Fatalf("%+v", lq.Sort)
	}
}
