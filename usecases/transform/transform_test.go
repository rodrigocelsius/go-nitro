package transform_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
)

func TestTitleCasePT(t *testing.T) {
	got := transform.TitleCase("joão da silva", transform.DefaultTitleCasePT())
	if got != "João da Silva" {
		t.Fatalf("got %q", got)
	}
}

// TestSnakeCase_GoldenVectors mirrors the same table asserted in
// codegen/data/naming_test.go and blinq/scan/cursor_test.go. This is the
// "no db/json tag" default naming used across the framework (db columns,
// JSON keys, BLinq metamodel); keep the copies of the algorithm in sync by
// running the same cases wherever one lives.
func TestSnakeCase_GoldenVectors(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"ID", "id"},
		{"SKU", "sku"},
		{"URL", "url"},
		{"UserID", "user_id"},
		{"OrderID", "order_id"},
		{"HTTPStatus", "http_status"},
		{"IDUser", "id_user"},
		{"Name", "name"},
		{"UserProfile", "user_profile"},
		{"APIKey", "api_key"},
		{"userID", "user_id"},
		{"A", "a"},
		{"CustomerName", "customer_name"},
		{"UnitPriceUSD", "unit_price_usd"},
		{"URLPath", "url_path"},
	}
	for _, tc := range cases {
		if got := transform.SnakeCase(tc.input); got != tc.want {
			t.Errorf("SnakeCase(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCases(t *testing.T) {
	if g := transform.CamelCase("foo_bar"); g != "fooBar" {
		t.Fatalf("camel %q", g)
	}
	if g := transform.PascalCase("foo_bar"); g != "FooBar" {
		t.Fatalf("pascal %q", g)
	}
	if g := transform.SnakeCase("FooBar"); g != "foo_bar" {
		t.Fatalf("snake %q", g)
	}
	if g := transform.KebabCase("FooBar"); g != "foo-bar" {
		t.Fatalf("kebab %q", g)
	}
	if g := transform.Digits("(11) 98877-6655"); g != "11988776655" {
		t.Fatalf("digits %q", g)
	}
}

func TestApplyTagPipeline(t *testing.T) {
	type Row struct {
		Name  string  `transform:"trim,titlecase=pt"`
		Phone string  `transform:"digits"`
		State string  `transform:"trim,upper"`
		Nick  *string `transform:"titlecase=pt"`
	}
	nick := " maria de souza "
	r := Row{
		Name:  "  joão da silva ",
		Phone: "(11) 9-8877",
		State: " sp ",
		Nick:  &nick,
	}
	transform.Apply(&r)
	if r.Name != "João da Silva" {
		t.Fatalf("name=%q", r.Name)
	}
	if r.Phone != "1198877" {
		t.Fatalf("phone=%q", r.Phone)
	}
	if r.State != "SP" {
		t.Fatalf("state=%q", r.State)
	}
	if r.Nick == nil || *r.Nick != "Maria de Souza" {
		t.Fatalf("nick=%v", r.Nick)
	}
}

func TestApplyNestedSlice(t *testing.T) {
	type Item struct {
		Label string `transform:"upper"`
	}
	type Box struct {
		Items []Item
	}
	b := Box{Items: []Item{{Label: "ab"}, {Label: "cd"}}}
	transform.Apply(&b)
	if b.Items[0].Label != "AB" || b.Items[1].Label != "CD" {
		t.Fatalf("%#v", b)
	}
}
