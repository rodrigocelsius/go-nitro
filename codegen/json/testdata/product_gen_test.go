package testdata

import (
	"testing"
	"time"
)

func TestProduct_Roundtrip_WithMapsAndTime(t *testing.T) {
	ts := time.Date(2025, 7, 12, 15, 30, 0, 0, time.UTC)
	original := Product{
		Name:       "Widget",
		Price:      29.99,
		Metadata:   map[string]string{"color": "red", "size": "large"},
		Quantities: map[string]int{"warehouse_a": 100, "warehouse_b": 50},
		CreatedAt:  ts,
	}

	buf := original.AppendJSON(nil)

	var decoded Product
	if !decoded.ParseJSON(buf) {
		t.Fatalf("ParseJSON failed for: %s", buf)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Price != original.Price {
		t.Errorf("Price: got %f, want %f", decoded.Price, original.Price)
	}
	if len(decoded.Metadata) != len(original.Metadata) {
		t.Fatalf("Metadata len: got %d, want %d", len(decoded.Metadata), len(original.Metadata))
	}
	for k, v := range original.Metadata {
		if decoded.Metadata[k] != v {
			t.Errorf("Metadata[%s]: got %q, want %q", k, decoded.Metadata[k], v)
		}
	}
	if len(decoded.Quantities) != len(original.Quantities) {
		t.Fatalf("Quantities len: got %d, want %d", len(decoded.Quantities), len(original.Quantities))
	}
	for k, v := range original.Quantities {
		if decoded.Quantities[k] != v {
			t.Errorf("Quantities[%s]: got %d, want %d", k, decoded.Quantities[k], v)
		}
	}
	if !decoded.CreatedAt.Equal(ts) {
		t.Errorf("CreatedAt: got %v, want %v", decoded.CreatedAt, ts)
	}
}

func TestProduct_NilMaps(t *testing.T) {
	original := Product{
		Name:  "Empty",
		Price: 0,
	}

	buf := original.AppendJSON(nil)

	var decoded Product
	if !decoded.ParseJSON(buf) {
		t.Fatalf("ParseJSON failed for: %s", buf)
	}

	if decoded.Name != "Empty" {
		t.Errorf("Name: got %q, want %q", decoded.Name, "Empty")
	}
	if decoded.Metadata != nil {
		t.Errorf("Metadata should be nil, got %v", decoded.Metadata)
	}
	if decoded.Quantities != nil {
		t.Errorf("Quantities should be nil, got %v", decoded.Quantities)
	}
}

func TestProduct_ArbitraryKeyOrder(t *testing.T) {
	data := []byte(`{"created_at":"2025-01-01T00:00:00Z","price":9.99,"quantities":{"a":1},"name":"Test","metadata":{"k":"v"}}`)

	var decoded Product
	if !decoded.ParseJSON(data) {
		t.Fatalf("ParseJSON failed for: %s", data)
	}

	if decoded.Name != "Test" {
		t.Errorf("Name: got %q, want %q", decoded.Name, "Test")
	}
	if decoded.Price != 9.99 {
		t.Errorf("Price: got %f, want 9.99", decoded.Price)
	}
	if decoded.Quantities["a"] != 1 {
		t.Errorf("Quantities[a]: got %d, want 1", decoded.Quantities["a"])
	}
	if decoded.Metadata["k"] != "v" {
		t.Errorf("Metadata[k]: got %q, want %q", decoded.Metadata["k"], "v")
	}
}

func BenchmarkGenerated_Marshal_Product(b *testing.B) {
	p := Product{
		Name:       "Widget",
		Price:      29.99,
		Metadata:   map[string]string{"color": "red"},
		Quantities: map[string]int{"a": 100},
		CreatedAt:  time.Now(),
	}
	buf := make([]byte, 0, 256)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = buf[:0]
		buf = p.AppendJSON(buf)
	}
}
