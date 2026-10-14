package jsontag_test

import (
	"encoding/json"
	"testing"

	"github.com/rodrigocelsius/go-nitro/usecases/v2/jsontag"
)

// fullyTagged has every field with an explicit json tag — the "old style".
// Marshal/Unmarshal here must be byte-identical to plain encoding/json
// (regression guard for the zero-overhead fast path).
type fullyTagged struct {
	Message string `json:"message"`
	Active  bool   `json:"active"`
}

func TestMarshal_FullyTagged_MatchesStdlib(t *testing.T) {
	v := fullyTagged{Message: "hi", Active: true}
	want, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jsontag.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %s, want %s (fully tagged types must be byte-identical to stdlib)", got, want)
	}
}

// noTags omits both json and db entirely on every field — the new style.
type noTags struct {
	Message      string
	CustomerName string
	UnitPriceUSD float64
}

func TestMarshal_NoTags_DefaultsToSnakeCase(t *testing.T) {
	v := noTags{Message: "hi", CustomerName: "Ada", UnitPriceUSD: 9.5}
	got, err := jsontag.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("marshal produced invalid json: %v (%s)", err, got)
	}
	for _, key := range []string{"message", "customer_name", "unit_price_usd"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing key %q in %s", key, got)
		}
	}
	if _, ok := m["CustomerName"]; ok {
		t.Errorf("unexpected PascalCase key leaked into %s", got)
	}
}

func TestUnmarshal_NoTags_AcceptsSnakeCase(t *testing.T) {
	data := []byte(`{"message":"hi","customer_name":"Ada","unit_price_usd":9.5}`)
	var v noTags
	if err := jsontag.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	if v.Message != "hi" || v.CustomerName != "Ada" || v.UnitPriceUSD != 9.5 {
		t.Fatalf("got %#v", v)
	}
}

func TestRoundTrip_NoTags(t *testing.T) {
	v := noTags{Message: "hi", CustomerName: "Ada", UnitPriceUSD: 9.5}
	data, err := jsontag.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got noTags
	if err := jsontag.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Fatalf("round-trip mismatch: got %#v, want %#v", got, v)
	}
}

// mixedTags: some fields tagged (old style), some not (new style), one
// db-only customization, one explicit exclusion.
type mixedTags struct {
	Message      string  `json:"message" db:"message"`
	CustomerName string  `validate:"required"`
	UnitPriceUSD float64 `db:"price_usd"`
	Secret       string  `json:"-"`
}

func TestRoundTrip_MixedTags(t *testing.T) {
	v := mixedTags{Message: "hi", CustomerName: "Ada", UnitPriceUSD: 9.5, Secret: "shh"}
	data, err := jsontag.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	if _, ok := m["Secret"]; ok {
		t.Fatalf("json:\"-\" field leaked: %s", data)
	}
	if _, ok := m["secret"]; ok {
		t.Fatalf("json:\"-\" field leaked: %s", data)
	}
	if m["message"] != "hi" || m["customer_name"] != "Ada" || m["unit_price_usd"] != 9.5 {
		t.Fatalf("unexpected shape: %s", data)
	}

	var got mixedTags
	if err := jsontag.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Message != v.Message || got.CustomerName != v.CustomerName || got.UnitPriceUSD != v.UnitPriceUSD {
		t.Fatalf("got %#v, want %#v", got, v)
	}
	if got.Secret != "" {
		t.Fatalf("json:\"-\" field should not be populated: %#v", got)
	}
}

// nested / embedded: anonymous embed flattens; nested struct and slice of
// structs recurse only when the child type itself needs renaming.
type embeddedBase struct {
	ID string
}

type parentWithChild struct {
	embeddedBase
	OrderID string
	Child   noTags
	Items   []noTags `json:"items"`
}

func TestRoundTrip_EmbeddedNestedAndSlice(t *testing.T) {
	v := parentWithChild{
		embeddedBase: embeddedBase{ID: "p1"},
		OrderID:      "o1",
		Child:        noTags{Message: "m", CustomerName: "Ada", UnitPriceUSD: 1},
		Items: []noTags{
			{Message: "m1", CustomerName: "A1", UnitPriceUSD: 1},
			{Message: "m2", CustomerName: "A2", UnitPriceUSD: 2},
		},
	}
	data, err := jsontag.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid json: %v (%s)", err, data)
	}
	for _, key := range []string{"id", "order_id", "child", "items"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("missing key %q in %s", key, data)
		}
	}
	child, ok := m["child"].(map[string]any)
	if !ok {
		t.Fatalf("child not an object: %s", data)
	}
	if _, ok := child["customer_name"]; !ok {
		t.Fatalf("nested struct key not renamed: %s", data)
	}
	items, ok := m["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("items not a 2-element array: %s", data)
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("item not an object: %s", data)
	}
	if _, ok := first["customer_name"]; !ok {
		t.Fatalf("slice element key not renamed: %s", data)
	}

	var got parentWithChild
	if err := jsontag.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != v.ID || got.OrderID != v.OrderID || got.Child != v.Child || len(got.Items) != 2 || got.Items[0] != v.Items[0] {
		t.Fatalf("round-trip mismatch: got %#v, want %#v", got, v)
	}
}

func TestMarshal_NonStruct_FallsBackToStdlib(t *testing.T) {
	got, err := jsontag.Marshal([]int{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "[1,2,3]" {
		t.Fatalf("got %s", got)
	}
}
