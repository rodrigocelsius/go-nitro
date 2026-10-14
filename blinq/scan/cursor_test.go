package scan

import (
	"reflect"
	"testing"
	"time"
)

type fakeCursor struct {
	columns []string
	rows    [][]any
	i       int
}

func (f *fakeCursor) Columns() []string {
	if f.columns != nil {
		return f.columns
	}
	return []string{"id", "full_name", "created_at", "score"}
}
func (f *fakeCursor) Next() bool { return f.i < len(f.rows) }
func (f *fakeCursor) Values() ([]any, error) {
	v := f.rows[f.i]
	f.i++
	return v, nil
}
func (f *fakeCursor) Err() error { return nil }
func (f *fakeCursor) Close()     {}

func TestScanCursorInto(t *testing.T) {
	ts := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	c := &fakeCursor{rows: [][]any{
		{1, "Ada Lovelace", ts, 9.5},
		{2, "Alan Turing", ts.Add(time.Hour), nil},
	}}

	type Row struct {
		ID        int       `db:"id"`
		FullName  string    `db:"full_name"`
		CreatedAt time.Time `db:"created_at"`
		Score     float64   `db:"score"`
	}
	var out []Row
	if err := ScanCursorInto[Row](c, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(out))
	}
	if out[0].ID != 1 || out[0].FullName != "Ada Lovelace" {
		t.Fatalf("%+v", out[0])
	}
	if !out[0].CreatedAt.Equal(ts) {
		t.Fatalf("time not preserved: %v", out[0].CreatedAt)
	}
	if out[1].Score != 0 { // NULL → zero value
		t.Fatalf("null handling: %+v", out[1])
	}
}

type embedBase struct {
	BaseCol string `db:"base_col"`
}

func TestScanCursorInto_Embedded(t *testing.T) {
	c := &fakeCursor{
		columns: []string{"base_col", "own"},
		rows:    [][]any{{"b1", "o1"}},
	}
	type WithEmbed struct {
		embedBase
		Own string `db:"own"`
	}
	var out []WithEmbed
	if err := ScanCursorInto[WithEmbed](c, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].BaseCol != "b1" || out[0].Own != "o1" {
		t.Fatalf("%+v", out)
	}
}

func TestBindingsOf_Cached(t *testing.T) {
	type sample struct {
		A string `db:"a"`
		B int
	}
	b1 := bindingsOf(reflect.TypeOf(sample{}))
	b2 := bindingsOf(reflect.TypeOf(sample{}))
	if b1 != b2 {
		t.Fatal("bindings must be memoized per type")
	}
	if b1.fields[0].column != "a" || b1.fields[1].column != "b" {
		t.Fatalf("%+v", b1.fields)
	}
}

func TestToSnakeCase_ASCII(t *testing.T) {
	tests := map[string]string{
		"UserID":      "user_id",
		"HTTPCode":    "http_code",
		"CreatedAt":   "created_at",
		"name":        "name",
		"CNPJNumber":  "cnpj_number",
		"TotalAmount": "total_amount",
	}
	for in, want := range tests {
		if got := ToSnakeCase(in); got != want {
			t.Errorf("ToSnakeCase(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestToSnakeCase_GoldenVectors mirrors the same acronym/compound-name cases
// asserted in codegen/data/naming_test.go and usecases/transform/transform_test.go.
// This is the "no db/json tag" default naming used across the framework
// (db columns, JSON keys, BLinq metamodel): keep the three implementations
// in sync by running the same table wherever a copy of the algorithm lives.
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
		if got := ToSnakeCase(tc.input); got != tc.want {
			t.Errorf("ToSnakeCase(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
