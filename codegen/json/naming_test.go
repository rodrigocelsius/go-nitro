package gen

import "testing"

// TestToSnakeCase_GoldenVectors mirrors the same table asserted in
// codegen/data/naming_test.go, blinq/scan/cursor_test.go and
// usecases/transform/transform_test.go. This is the "no json/db tag"
// default naming used across the framework; keep the copies of the
// algorithm in sync by running the same cases wherever one lives.
func TestToSnakeCase_GoldenVectors(t *testing.T) {
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
		if got := toSnakeCase(tc.input); got != tc.want {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
