package domain_test

import (
	"database/sql/driver"
	"encoding/json"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestJSONSlice_RoundTrip(t *testing.T) {
	s := domain.StringSlice{"a", "b"}
	v, err := s.Value()
	if err != nil {
		t.Fatal(err)
	}
	b, ok := v.([]byte)
	if !ok {
		t.Fatalf("%T", v)
	}
	var out domain.StringSlice
	if err := out.Scan(b); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0] != "a" {
		t.Fatalf("%#v", out)
	}
}

func TestJSONSlice_NilValue(t *testing.T) {
	var s domain.StringSlice
	v, err := s.Value()
	if err != nil {
		t.Fatal(err)
	}
	if string(v.([]byte)) != "[]" {
		t.Fatalf("%s", v)
	}
	var out domain.StringSlice
	if err := out.Scan(nil); err != nil {
		t.Fatal(err)
	}
	if out == nil || len(out) != 0 {
		t.Fatalf("%#v", out)
	}
}

func TestJSONSlice_HTTPJSON(t *testing.T) {
	type Row struct {
		Tags domain.StringSlice `json:"tags"`
	}
	var r Row
	if err := json.Unmarshal([]byte(`{"tags":["x","y"]}`), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Tags) != 2 {
		t.Fatalf("%#v", r)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"tags":["x","y"]}` {
		t.Fatalf("%s", b)
	}
}

func TestJSON_Object(t *testing.T) {
	j := domain.JSON[map[string]int]{V: map[string]int{"n": 1}}
	v, err := j.Value()
	if err != nil {
		t.Fatal(err)
	}
	_ = driver.Value(v)
	var out domain.JSON[map[string]int]
	if err := out.Scan(v); err != nil {
		t.Fatal(err)
	}
	if out.V["n"] != 1 {
		t.Fatalf("%#v", out)
	}
	b, _ := json.Marshal(j)
	if string(b) != `{"n":1}` {
		t.Fatalf("%s", b)
	}
}

func TestJSONSlice_GenericStruct(t *testing.T) {
	type Item struct {
		Day string `json:"day"`
	}
	s := domain.JSONSlice[Item]{{Day: "mon"}}
	v, err := s.Value()
	if err != nil {
		t.Fatal(err)
	}
	var out domain.JSONSlice[Item]
	if err := out.Scan(v); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Day != "mon" {
		t.Fatalf("%#v", out)
	}
}
