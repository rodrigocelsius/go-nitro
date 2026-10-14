package runtime

import (
	"encoding/json"
	"testing"
)

func TestAppendStringField(t *testing.T) {
	var buf []byte
	buf = AppendStringField(buf, "name", "John")
	if string(buf) != `"name":"John"` {
		t.Errorf("got %s, want \"name\":\"John\"", buf)
	}
}

func TestAppendIntField(t *testing.T) {
	var buf []byte
	buf = AppendIntField(buf, "age", 30)
	if string(buf) != `"age":30` {
		t.Errorf("got %s, want \"age\":30", buf)
	}
}

func TestAppendFloat64Field(t *testing.T) {
	var buf []byte
	buf = AppendFloat64Field(buf, "price", 9.99)
	if string(buf) != `"price":9.99` {
		t.Errorf("got %s, want \"price\":9.99", buf)
	}
}

func TestAppendBoolField(t *testing.T) {
	var buf []byte
	buf = AppendBoolField(buf, "active", true)
	if string(buf) != `"active":true` {
		t.Errorf("got %s, want \"active\":true", buf)
	}
}

func TestAppendNullField(t *testing.T) {
	var buf []byte
	buf = AppendNullField(buf, "deleted")
	if string(buf) != `"deleted":null` {
		t.Errorf("got %s, want \"deleted\":null", buf)
	}
}

func TestAppendMultipleFields(t *testing.T) {
	var buf []byte
	buf = AppendObjectStart(buf)
	buf = AppendStringField(buf, "name", "John")
	buf = AppendComma(buf)
	buf = AppendIntField(buf, "age", 30)
	buf = AppendComma(buf)
	buf = AppendBoolField(buf, "active", true)
	buf = AppendObjectEnd(buf)

	expected := `{"name":"John","age":30,"active":true}`
	if string(buf) != expected {
		t.Errorf("got %s, want %s", buf, expected)
	}
}

func TestAppendStringField_Escaped(t *testing.T) {
	var buf []byte
	buf = AppendStringField(buf, "msg", `Say "hi" \ there`)
	result := string(buf)
	if result != `"msg":"Say \"hi\" \\ there"` {
		t.Errorf("got %s", result)
	}
}

func TestFastWrite_Roundtrip(t *testing.T) {
	var buf []byte
	buf = AppendObjectStart(buf)
	buf = AppendStringField(buf, "name", "Test")
	buf = AppendComma(buf)
	buf = AppendIntField(buf, "count", 42)
	buf = AppendComma(buf)
	buf = AppendFloat64Field(buf, "score", 3.14)
	buf = AppendComma(buf)
	buf = AppendBoolField(buf, "ok", true)
	buf = AppendComma(buf)
	buf = AppendNullField(buf, "nothing")
	buf = AppendObjectEnd(buf)

	var m map[string]interface{}
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if m["name"] != "Test" {
		t.Errorf("name: got %v, want Test", m["name"])
	}
	if m["count"] != float64(42) {
		t.Errorf("count: got %v, want 42", m["count"])
	}
	if m["ok"] != true {
		t.Errorf("ok: got %v, want true", m["ok"])
	}
	if m["nothing"] != nil {
		t.Errorf("nothing: got %v, want nil", m["nothing"])
	}
}

func BenchmarkFastWrite(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf []byte
		buf = AppendObjectStart(buf)
		buf = AppendStringField(buf, "name", "John")
		buf = AppendComma(buf)
		buf = AppendIntField(buf, "age", 30)
		buf = AppendComma(buf)
		buf = AppendFloat64Field(buf, "score", 9.5)
		buf = AppendComma(buf)
		buf = AppendBoolField(buf, "active", true)
		buf = AppendObjectEnd(buf)
	}
}

func BenchmarkSprintf(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := `{"name":"John","age":30,"score":9.5,"active":true}`
		_ = s
	}
}

func BenchmarkAppendVsSprintf(b *testing.B) {
	b.Run("Append", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var buf []byte
			buf = AppendObjectStart(buf)
			buf = AppendStringField(buf, "name", "John")
			buf = AppendComma(buf)
			buf = AppendIntField(buf, "age", 30)
			buf = AppendObjectEnd(buf)
		}
	})
}
