package nitro

import "testing"

type pathProduct struct {
	Name string `json:"name" db:"name" blinq:"index"`
}

func (pathProduct) TableName() string { return "catalog_items" }

func TestResourcePathFromModel(t *testing.T) {
	if got := resourcePathFromModel(&pathProduct{}); got != "catalog_items" {
		t.Fatalf("TableName: got %q", got)
	}
	type OrderItem struct{}
	if got := resourcePathFromModel(&OrderItem{}); got != "order_items" {
		t.Fatalf("plural: got %q", got)
	}
}

func TestInferResourceFields(t *testing.T) {
	type row struct {
		ID     string  `json:"id" db:"id"`
		Name   string  `json:"name" db:"name" blinq:"index"`
		Active bool    `json:"active" db:"active" blinq:"index"`
		Price  float64 `json:"price" db:"price"`
	}
	f, s := inferResourceFields(&row{})
	if len(f) != 2 || f[0] != "name" || f[1] != "active" {
		t.Fatalf("filterable=%v", f)
	}
	if len(s) < 3 {
		t.Fatalf("sortable=%v", s)
	}
}
