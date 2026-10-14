package testdata

import (
	"encoding/json"
	"testing"
)

// Same Order type, but marshal/unmarshal via standard encoding/json,
// for a direct comparison with generated code (AppendJSON/ParseJSON).
// Identical structs — only the serialization method changes.

type OrderStdlib struct {
	ID      int            `json:"id"`
	Total   float64        `json:"total"`
	Active  bool           `json:"active"`
	Address AddressStdlib  `json:"address"`
	Billing *AddressStdlib `json:"billing"`
	Items   []ItemStdlib   `json:"items"`
	Tags    []string       `json:"tags"`
}

type AddressStdlib struct {
	Street string `json:"street"`
	City   string `json:"city"`
}

type ItemStdlib struct {
	SKU   string  `json:"sku"`
	Price float64 `json:"price"`
}

func sampleOrder() Order {
	return Order{
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
}

func sampleOrderStdlib() OrderStdlib {
	return OrderStdlib{
		ID:      1,
		Total:   99.5,
		Active:  true,
		Address: AddressStdlib{Street: "Rua A", City: "SP"},
		Billing: &AddressStdlib{Street: "Rua B", City: "RJ"},
		Items: []ItemStdlib{
			{SKU: "X1", Price: 10.5},
			{SKU: "X2", Price: 20.25},
		},
		Tags: []string{"urgent", "gift"},
	}
}

func BenchmarkGenerated_Marshal_Order(b *testing.B) {
	o := sampleOrder()
	buf := make([]byte, 0, 512)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = buf[:0]
		buf = o.AppendJSON(buf)
	}
}

func BenchmarkStdlib_Marshal_Order(b *testing.B) {
	o := sampleOrderStdlib()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(o)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerated_Unmarshal_Order(b *testing.B) {
	o := sampleOrder()
	data := o.AppendJSON(nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded Order
		if !decoded.ParseJSON(data) {
			b.Fatal("parse failed")
		}
	}
}

func BenchmarkStdlib_Unmarshal_Order(b *testing.B) {
	o := sampleOrderStdlib()
	data, _ := json.Marshal(o)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded OrderStdlib
		if err := json.Unmarshal(data, &decoded); err != nil {
			b.Fatal(err)
		}
	}
}
