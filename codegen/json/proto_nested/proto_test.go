package proto_nested

import "testing"

func TestOrderParseJSON_A_Correctness(t *testing.T) {
	data := []byte(`{"id":5,"address":{"street":"Rua X","city":"SP"}}`)
	var o Order
	if !o.OrderParseJSON_A(data) {
		t.Fatal("parse failed")
	}
	if o.ID != 5 || o.Address.Street != "Rua X" || o.Address.City != "SP" {
		t.Fatalf("unexpected result: %+v", o)
	}
}

func TestOrderParseJSON_B_Correctness(t *testing.T) {
	data := []byte(`{"id":5,"address":{"street":"Rua X","city":"SP"}}`)
	var o Order
	if !o.OrderParseJSON_B(data) {
		t.Fatal("parse failed")
	}
	if o.ID != 5 || o.Address.Street != "Rua X" || o.Address.City != "SP" {
		t.Fatalf("unexpected result: %+v", o)
	}
}

func TestOrderParseJSON_B_UnknownFieldsAfterNested(t *testing.T) {
	// Ensure that after parsing the nested field via B (no prior
	// SkipValue), the parser is at the right position to keep reading
	// the following fields of the parent object.
	data := []byte(`{"id":5,"address":{"street":"Rua X","city":"SP","extra":"ignored"},"trailing":"ok"}`)
	var o Order
	if !o.OrderParseJSON_B(data) {
		t.Fatal("parse failed")
	}
	if o.ID != 5 || o.Address.Street != "Rua X" {
		t.Fatalf("unexpected result: %+v", o)
	}
}

func BenchmarkOrderParseJSON_A_SkipValueFirst(b *testing.B) {
	data := []byte(`{"id":5,"address":{"street":"Rua X longer street name","city":"Sao Paulo"}}`)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var o Order
		if !o.OrderParseJSON_A(data) {
			b.Fatal("parse failed")
		}
	}
}

func BenchmarkOrderParseJSON_B_DirectContinuation(b *testing.B) {
	data := []byte(`{"id":5,"address":{"street":"Rua X longer street name","city":"Sao Paulo"}}`)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var o Order
		if !o.OrderParseJSON_B(data) {
			b.Fatal("parse failed")
		}
	}
}
