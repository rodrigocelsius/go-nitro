package main

import "testing"

func TestNormalizeResourceName(t *testing.T) {
	cases := []struct {
		in, typ, pkg, table string
	}{
		{"Product", "Product", "product", "products"},
		{"products", "Product", "product", "products"},
		{"Products", "Product", "product", "products"},
		{"product", "Product", "product", "products"},
		{"OrderItem", "OrderItem", "orderitem", "order_items"},
		{"categories", "Category", "category", "categories"},
	}
	for _, tc := range cases {
		got, err := normalizeResourceName(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		if got.Type != tc.typ || got.Package != tc.pkg || got.Table != tc.table {
			t.Errorf("%s: got %+v want type=%s pkg=%s table=%s", tc.in, got, tc.typ, tc.pkg, tc.table)
		}
	}
}

func TestValidateProjectName(t *testing.T) {
	if err := validateProjectName("myapp"); err != nil {
		t.Fatal(err)
	}
	if err := validateProjectName("/tmp/x"); err == nil {
		t.Fatal("expected path error")
	}
	if err := validateProjectName("9bad"); err == nil {
		t.Fatal("expected invalid start")
	}
}

func TestSingularize(t *testing.T) {
	if got := singularizePascal("Categories"); got != "Category" {
		t.Fatalf("got %s", got)
	}
	if got := singularizePascal("Bus"); got != "Bus" {
		t.Fatalf("got %s (ss kept)", got)
	}
}
