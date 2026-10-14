package datagen

import "testing"

func TestTableNameFromStruct(t *testing.T) {
	cases := []struct {
		structName string
		want       string
	}{
		{"User", "users"},
		{"Order", "orders"},
		{"Category", "categories"},
		{"Address", "addresses"},
		{"Box", "boxes"},
		{"Bus", "buses"},
		{"Wish", "wishes"},
		{"Match", "matches"},
		{"Day", "days"},
		{"Company", "companies"},
		{"UserProfile", "user_profiles"},
		{"OrderItem", "order_items"},
	}

	for _, tc := range cases {
		got := tableNameFromStruct(tc.structName)
		if got != tc.want {
			t.Errorf("tableNameFromStruct(%q) = %q, want %q", tc.structName, got, tc.want)
		}
	}
}

func TestToSnakeCase_AcronymHandling(t *testing.T) {
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
	}

	for _, tc := range cases {
		got := toSnakeCase(tc.input)
		if got != tc.want {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestToSnakeCase_GoldenVectors mirrors the same table asserted in
// blinq/scan/cursor_test.go and usecases/transform/transform_test.go. This
// is the "no db/json tag" default naming used across the framework (db
// columns, JSON keys, BLinq metamodel); keep the copies of the algorithm in
// sync by running the same cases wherever one lives.
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
