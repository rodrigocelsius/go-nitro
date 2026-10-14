package domain

import (
	"reflect"
	"testing"
)

type fcBase struct{ ID string }
type fcModel struct {
	fcBase
	Name string
}
type fcPtr struct {
	*fcBase
	Name string
}

func TestFieldByNameCached(t *testing.T) {
	m := fcModel{fcBase: fcBase{ID: "x"}, Name: "n"}
	v := reflect.ValueOf(&m).Elem()
	for i := 0; i < 2; i++ { // second pass hits the cache
		if got := FieldByName(v, "ID"); got.String() != "x" || !got.CanSet() {
			t.Fatalf("promoted ID = %v", got)
		}
		if got := FieldByName(v, "Name"); got.String() != "n" {
			t.Fatalf("Name = %v", got)
		}
		if FieldByName(v, "Missing").IsValid() {
			t.Fatal("missing field must be invalid")
		}
	}
	// nil embedded pointer: reflect would panic, the helper returns invalid.
	if FieldByName(reflect.ValueOf(fcPtr{}), "ID").IsValid() {
		t.Fatal("field behind nil embedded pointer must be invalid")
	}
	if FieldByName(reflect.ValueOf(42), "ID").IsValid() || FieldByName(reflect.Value{}, "ID").IsValid() {
		t.Fatal("non-struct must be invalid")
	}
}

func BenchmarkFieldByName(b *testing.B) {
	v := reflect.ValueOf(&fcModel{}).Elem()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = FieldByName(v, "ID")
	}
}
