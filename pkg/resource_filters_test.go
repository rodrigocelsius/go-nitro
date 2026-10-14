package nitro_test

import (
	"strings"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type filterItem struct {
	nitro.Model
	Name      string
	UnitPrice float64
}

func TestResource_MemoryFiltersOperatorsAndSort(t *testing.T) {
	app := nitro.New(nitro.Config{DisableDefaults: true, Env: nitro.EnvDevelopment})
	app.Resource(&filterItem{}, nitro.NewMemoryRepo[filterItem](), &nitro.ResourceConfig{
		Public: true, NoCache: true,
		Filterable:  []string{"name", "unit_price"},
		Sortable:    []string{"unit_price", "name"},
		FilterRules: []nitro.FilterRule{{Param: "q", Field: "name", Op: "icontains"}},
	})
	for _, b := range []string{`{"name":"Pen","unit_price":9}`, `{"name":"Pencil","unit_price":10}`, `{"name":"Desk","unit_price":120}`} {
		r, _ := app.TestWithHeaders("POST", "/filter_items", strings.NewReader(b), map[string]string{"Content-Type": "application/json"})
		if r.StatusCode() != 201 {
			t.Fatalf("create %d %s", r.StatusCode(), r.Body())
		}
	}
	get := func(q string) string {
		r, _ := app.Test("GET", "/filter_items"+q, nil)
		return string(r.Body())
	}
	if b := get("?unit_price_gte=10"); strings.Contains(b, `"Pen"`) && !strings.Contains(b, `"Pencil"`) || !strings.Contains(b, "Desk") {
		t.Fatalf("gte: %s", b)
	}
	if b := get("?unit_price_gte=10&unit_price_lt=100"); !strings.Contains(b, "Pencil") || strings.Contains(b, "Desk") || strings.Contains(b, `"name":"Pen"`) {
		t.Fatalf("range: %s", b)
	}
	if b := get("?q=pen"); !strings.Contains(b, "Pen") || !strings.Contains(b, "Pencil") || strings.Contains(b, "Desk") {
		t.Fatalf("icontains rule: %s", b)
	}
	// numeric sort: 9 < 10 < 120 (lexicographic would put 120 before 9)
	b := get("?sort=unit_price")
	if !(strings.Index(b, "Pen\"") < strings.Index(b, "Pencil") && strings.Index(b, "Pencil") < strings.Index(b, "Desk")) {
		t.Fatalf("numeric sort: %s", b)
	}
}
