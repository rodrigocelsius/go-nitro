package runtime

import (
	"encoding/json"
	"sync"
	"testing"
)

type testProduct struct {
	Name  string `json:"name"`
	Price int    `json:"price"`
}

func (v *testProduct) AppendJSON(buf []byte) []byte {
	buf = AppendObjectStart(buf)
	buf = AppendStringField(buf, "name", v.Name)
	buf = AppendComma(buf)
	buf = AppendIntField(buf, "price", v.Price)
	buf = AppendObjectEnd(buf)
	return buf
}

func TestGetBuffer_PutBuffer(t *testing.T) {
	buf := GetBuffer()
	if buf == nil {
		t.Fatal("GetBuffer returned nil")
	}

	*buf = append(*buf, "test"...)
	if len(*buf) != 4 {
		t.Fatalf("len(buf) = %d, want 4", len(*buf))
	}

	PutBuffer(buf)

	buf2 := GetBuffer()
	if len(*buf2) != 0 {
		t.Errorf("buffer not reset: len = %d, want 0", len(*buf2))
	}
	PutBuffer(buf2)
}

func TestPutBuffer_Nil(t *testing.T) {
	PutBuffer(nil) // should not panic
}

func TestMarshalWithPool(t *testing.T) {
	p := &testProduct{Name: "Widget", Price: 29}

	result := MarshalWithPool(p)
	if len(result) == 0 {
		t.Fatal("MarshalWithPool returned empty")
	}

	var decoded testProduct
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if decoded.Name != "Widget" || decoded.Price != 29 {
		t.Errorf("decoded: %+v, want {Widget 29}", decoded)
	}
}

func TestMarshalWithPool_SafeCopy(t *testing.T) {
	p := &testProduct{Name: "A", Price: 1}
	r1 := MarshalWithPool(p)

	p2 := &testProduct{Name: "B", Price: 2}
	r2 := MarshalWithPool(p2)

	// r1 should not be affected by r2
	if string(r1) != `{"name":"A","price":1}` {
		t.Errorf("r1 corrupted: %s", r1)
	}
	if string(r2) != `{"name":"B","price":2}` {
		t.Errorf("r2 corrupted: %s", r2)
	}
}

func TestMarshalWithPool_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			p := &testProduct{Name: "test", Price: n}
			result := MarshalWithPool(p)
			if len(result) == 0 {
				t.Errorf("empty result for %d", n)
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkMarshalWithPool(b *testing.B) {
	p := &testProduct{Name: "Widget", Price: 29}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MarshalWithPool(p)
	}
}

func BenchmarkMarshalWithoutPool(b *testing.B) {
	p := &testProduct{Name: "Widget", Price: 29}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.AppendJSON(nil)
	}
}
