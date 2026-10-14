package testdata

import (
	"strings"
	"testing"
)

func TestGeneratedOrder_Roundtrip_WithBilling(t *testing.T) {
	original := Order{
		ID:      1,
		Total:   99.5,
		Active:  true,
		Address: Address{Street: "Rua A", City: "SP"},
		Billing: &Address{Street: "Rua B", City: "RJ"},
		Items: []Item{
			{SKU: "X1", Price: 10.5},
			{SKU: "X2", Price: 20.25},
		},
		Tags: []string{"urgent", "gift"},
	}

	buf := original.AppendJSON(nil)

	var decoded Order
	if !decoded.ParseJSON(buf) {
		t.Fatalf("ParseJSON failed for: %s", buf)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, original.ID)
	}
	if decoded.Total != original.Total {
		t.Errorf("Total mismatch: got %f, want %f", decoded.Total, original.Total)
	}
	if decoded.Active != original.Active {
		t.Errorf("Active mismatch: got %v, want %v", decoded.Active, original.Active)
	}
	if decoded.Address != original.Address {
		t.Errorf("Address mismatch: got %+v, want %+v", decoded.Address, original.Address)
	}
	if decoded.Billing == nil {
		t.Fatal("Billing should not be nil")
	}
	if *decoded.Billing != *original.Billing {
		t.Errorf("Billing mismatch: got %+v, want %+v", *decoded.Billing, *original.Billing)
	}
	if len(decoded.Items) != len(original.Items) {
		t.Fatalf("Items length mismatch: got %d, want %d", len(decoded.Items), len(original.Items))
	}
	for i := range original.Items {
		if decoded.Items[i] != original.Items[i] {
			t.Errorf("Items[%d] mismatch: got %+v, want %+v", i, decoded.Items[i], original.Items[i])
		}
	}
	if len(decoded.Tags) != len(original.Tags) {
		t.Fatalf("Tags length mismatch: got %d, want %d", len(decoded.Tags), len(original.Tags))
	}
	for i := range original.Tags {
		if decoded.Tags[i] != original.Tags[i] {
			t.Errorf("Tags[%d] mismatch: got %q, want %q", i, decoded.Tags[i], original.Tags[i])
		}
	}
}

func TestGeneratedOrder_Roundtrip_NilBilling(t *testing.T) {
	original := Order{
		ID:      2,
		Total:   0,
		Active:  false,
		Address: Address{Street: "Rua C", City: "BH"},
		Billing: nil,
		Items:   nil,
		Tags:    nil,
	}

	buf := original.AppendJSON(nil)

	var decoded Order
	if !decoded.ParseJSON(buf) {
		t.Fatalf("ParseJSON failed for: %s", buf)
	}
	if decoded.Billing != nil {
		t.Fatalf("expected nil Billing, got %+v", decoded.Billing)
	}
	if len(decoded.Items) != 0 {
		t.Fatalf("expected empty Items, got %+v", decoded.Items)
	}
	if len(decoded.Tags) != 0 {
		t.Fatalf("expected empty Tags, got %+v", decoded.Tags)
	}
}

func TestGeneratedOrder_ParseJSON_ArbitraryKeyOrder(t *testing.T) {
	// Assert the parser does NOT depend on JSON key order — unlike the
	// initial prototype (user_manual.go), which assumed a fixed order.
	// That is essential for a general-purpose generator.
	data := []byte(`{"active":true,"id":5,"tags":["a","b"],"total":1.5,"address":{"city":"X","street":"Y"},"items":[],"billing":null}`)

	var decoded Order
	if !decoded.ParseJSON(data) {
		t.Fatalf("ParseJSON failed for: %s", data)
	}
	if decoded.ID != 5 || decoded.Total != 1.5 || !decoded.Active {
		t.Errorf("unexpected result: %+v", decoded)
	}
	if decoded.Address.City != "X" || decoded.Address.Street != "Y" {
		t.Errorf("unexpected address: %+v", decoded.Address)
	}
	if decoded.Billing != nil {
		t.Errorf("expected nil billing, got %+v", decoded.Billing)
	}
}

func TestGeneratedOrder_ParseJSON_UnknownFieldsIgnored(t *testing.T) {
	data := []byte(`{"id":9,"total":0,"active":false,"address":{"street":"S","city":"C"},"billing":null,"items":[],"tags":[],"extra_field":"should be ignored","another":{"nested":true}}`)

	var decoded Order
	if !decoded.ParseJSON(data) {
		t.Fatalf("ParseJSON failed for: %s", data)
	}
	if decoded.ID != 9 {
		t.Errorf("expected ID 9, got %d", decoded.ID)
	}
}

func TestGeneratedOrder_UntaggedFieldDefaultsToSnakeCase(t *testing.T) {
	original := Order{ID: 1, Internal: "secret-ish"}
	buf := original.AppendJSON(nil)
	if !strings.Contains(string(buf), `"internal"`) {
		t.Fatalf("expected default key \"internal\" in generated JSON: %s", buf)
	}

	var decoded Order
	if !decoded.ParseJSON([]byte(`{"id":1,"internal":"secret-ish"}`)) {
		t.Fatalf("ParseJSON failed")
	}
	if decoded.Internal != "secret-ish" {
		t.Fatalf("Internal not decoded: %+v", decoded)
	}
}

func TestGeneratedAddress_EscapedStrings(t *testing.T) {
	original := Address{Street: `Say "hi" \ back`, City: "Line\nBreak"}
	buf := original.AppendJSON(nil)

	var decoded Address
	if !decoded.ParseJSON(buf) {
		t.Fatalf("ParseJSON failed for: %s", buf)
	}
	if decoded != original {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}
