package nitro

import (
	"sort"
	"testing"
)

func TestSafeMap_KeysClear(t *testing.T) {
	m := NewSafeMap[string, int]()
	m.Set("b", 2)
	m.Set("a", 1)
	keys := m.Keys()
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Fatalf("Keys = %v", keys)
	}
	if m.Len() != 2 {
		t.Fatalf("Len = %d", m.Len())
	}
	m.Clear()
	if m.Len() != 0 {
		t.Fatalf("after Clear Len = %d", m.Len())
	}
	if len(m.Keys()) != 0 {
		t.Fatalf("after Clear Keys = %v", m.Keys())
	}
	if _, ok := m.Get("a"); ok {
		t.Fatal("Get after Clear should miss")
	}
}

func TestSafeMap_ValuesHasDelete(t *testing.T) {
	m := NewSafeMap[string, string]()
	m.Set("k", "v")
	if !m.Has("k") {
		t.Fatal("Has")
	}
	vals := m.Values()
	if len(vals) != 1 || vals[0] != "v" {
		t.Fatalf("Values = %v", vals)
	}
	m.Delete("k")
	if m.Has("k") {
		t.Fatal("Delete failed")
	}
}
