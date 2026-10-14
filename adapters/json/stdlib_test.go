package json

import (
	"testing"
)

func TestNewStdlibSerializer(t *testing.T) {
	s := NewStdlibSerializer()
	if s == nil {
		t.Fatal("NewStdlibSerializer returned nil")
	}
}

func TestStdlibSerializer_MarshalJSON(t *testing.T) {
	s := NewStdlibSerializer()

	data := map[string]string{"key": "value"}
	result, err := s.MarshalJSON(data)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	expected := `{"key":"value"}`
	if string(result) != expected {
		t.Errorf("MarshalJSON = %q, want %q", string(result), expected)
	}
}

func TestStdlibSerializer_MarshalJSON_Struct(t *testing.T) {
	s := NewStdlibSerializer()

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	u := User{Name: "Alice", Age: 30}
	result, err := s.MarshalJSON(u)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	expected := `{"name":"Alice","age":30}`
	if string(result) != expected {
		t.Errorf("MarshalJSON = %q, want %q", string(result), expected)
	}
}

func TestStdlibSerializer_MarshalJSON_Slice(t *testing.T) {
	s := NewStdlibSerializer()

	data := []int{1, 2, 3}
	result, err := s.MarshalJSON(data)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	expected := `[1,2,3]`
	if string(result) != expected {
		t.Errorf("MarshalJSON = %q, want %q", string(result), expected)
	}
}

func TestStdlibSerializer_UnmarshalJSON(t *testing.T) {
	s := NewStdlibSerializer()

	input := []byte(`{"name":"Bob","age":25}`)
	var result map[string]any
	err := s.UnmarshalJSON(input, &result)
	if err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	if result["name"] != "Bob" {
		t.Errorf("name = %v, want Bob", result["name"])
	}
}

func TestStdlibSerializer_UnmarshalJSON_Struct(t *testing.T) {
	s := NewStdlibSerializer()

	type Config struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}

	input := []byte(`{"host":"localhost","port":8080}`)
	var cfg Config
	err := s.UnmarshalJSON(input, &cfg)
	if err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want localhost", cfg.Host)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
}

func TestStdlibSerializer_UnmarshalJSON_InvalidJSON(t *testing.T) {
	s := NewStdlibSerializer()

	input := []byte(`{invalid}`)
	var result map[string]string
	err := s.UnmarshalJSON(input, &result)
	if err == nil {
		t.Error("UnmarshalJSON should return error for invalid JSON")
	}
}

func TestStdlibSerializer_Roundtrip(t *testing.T) {
	s := NewStdlibSerializer()

	original := map[string]any{
		"string": "hello",
		"number": 42.0,
		"bool":   true,
		"nested": map[string]any{"key": "val"},
	}

	data, err := s.MarshalJSON(original)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	var decoded map[string]any
	err = s.UnmarshalJSON(data, &decoded)
	if err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	if decoded["string"] != "hello" {
		t.Errorf("string = %v, want hello", decoded["string"])
	}
	if decoded["number"] != 42.0 {
		t.Errorf("number = %v, want 42", decoded["number"])
	}
	if decoded["bool"] != true {
		t.Errorf("bool = %v, want true", decoded["bool"])
	}
}

func BenchmarkStdlibSerializer_Marshal(b *testing.B) {
	s := NewStdlibSerializer()
	data := map[string]string{"name": "test", "value": "123"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = s.MarshalJSON(data)
	}
}

func BenchmarkStdlibSerializer_Unmarshal(b *testing.B) {
	s := NewStdlibSerializer()
	input := []byte(`{"name":"test","value":"123"}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var result map[string]string
		_ = s.UnmarshalJSON(input, &result)
	}
}
