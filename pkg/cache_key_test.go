package nitro

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestListQueryCacheKey_DistinguishesQueries(t *testing.T) {
	q := func(f ...domain.ListFilter) domain.ListQuery {
		return domain.ListQuery{Page: 1, PerPage: 20, Filters: f}
	}
	keys := map[string]string{}
	for name, lq := range map[string]domain.ListQuery{
		"gte":       q(domain.ListFilter{Field: "price", Value: "10", Op: "gte"}),
		"lt":        q(domain.ListFilter{Field: "price", Value: "10", Op: "lt"}),
		"eq":        q(domain.ListFilter{Field: "price", Value: "10", Op: "eq"}),
		"split-a":   q(domain.ListFilter{Field: "a", Value: "bc", Op: "eq"}),
		"split-b":   q(domain.ListFilter{Field: "ab", Value: "c", Op: "eq"}),
		"tenant-1":  q(domain.ListFilter{Field: "tenant_id", Value: "t1", Op: "eq"}),
		"tenant-2":  q(domain.ListFilter{Field: "tenant_id", Value: "t2", Op: "eq"}),
		"sort-asc":  {Page: 1, PerPage: 20, Sort: []domain.ListSort{{Field: "name"}}},
		"sort-desc": {Page: 1, PerPage: 20, Sort: []domain.ListSort{{Field: "name", Desc: true}}},
		"page-2":    {Page: 2, PerPage: 20},
	} {
		k := listQueryCacheKey(lq)
		if other, dup := keys[k]; dup {
			t.Errorf("%s and %s share cache key %s", name, other, k)
		}
		keys[k] = name
	}
	a := listQueryCacheKey(q(domain.ListFilter{Field: "price", Value: "10", Op: "GTE"}))
	b := listQueryCacheKey(q(domain.ListFilter{Field: "price", Value: "10", Op: "gte"}))
	if a != b {
		t.Error("operator case must not change the key")
	}
}
