package blinqgen

import "testing"

func TestTypeQualifiers(t *testing.T) {
	got := typeQualifiers("map[string]money.Money")
	if len(got) != 1 || got[0] != "money" {
		t.Fatalf("typeQualifiers = %v", got)
	}
	if got := typeQualifiers("*time.Time"); len(got) != 1 || got[0] != "time" {
		t.Fatalf("typeQualifiers = %v", got)
	}
	if got := typeQualifiers("[]string"); len(got) != 0 {
		t.Fatalf("typeQualifiers = %v", got)
	}
}

func TestHasLocalTypeRef(t *testing.T) {
	cases := map[string]bool{
		"map[string]Address": true,
		"*Status":            true,
		"[]string":           false,
		"*time.Time":         false,
		"map[string]any":     false,
		"money.Money":        false,
	}
	for in, want := range cases {
		if got := hasLocalTypeRef(in); got != want {
			t.Errorf("hasLocalTypeRef(%q) = %v, want %v", in, got, want)
		}
	}
}
