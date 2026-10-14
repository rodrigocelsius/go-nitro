package transform_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
)

func TestIsDigits(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"0", true},
		{"123456", true},
		{"12a", false},
		{" 1", false},
		{"١٢", false},
	}
	for _, tt := range tests {
		if got := transform.IsDigits(tt.in); got != tt.want {
			t.Errorf("IsDigits(%q)=%v want %v", tt.in, got, tt.want)
		}
	}
}

func TestSplitName(t *testing.T) {
	first, last := transform.SplitName("  Ana da Silva ")
	if first != "Ana" || last != "da Silva" {
		t.Fatalf("got %q %q", first, last)
	}
	first, last = transform.SplitName("Ana")
	if first != "Ana" || last != "" {
		t.Fatalf("single: %q %q", first, last)
	}
	first, last = transform.SplitName("   ")
	if first != "" || last != "" {
		t.Fatalf("blank: %q %q", first, last)
	}
}

func TestNormalizeEmail(t *testing.T) {
	if g := transform.NormalizeEmail("  Foo.Bar@Example.COM "); g != "foo.bar@example.com" {
		t.Fatalf("got %q", g)
	}
}

func TestIfBlank(t *testing.T) {
	if g := transform.IfBlank("  ", "x"); g != "x" {
		t.Fatalf("got %q", g)
	}
	if g := transform.IfBlank("ok", "x"); g != "ok" {
		t.Fatalf("got %q", g)
	}
}

func TestRandomDigits(t *testing.T) {
	s, err := transform.RandomDigits(6)
	if err != nil {
		t.Fatal(err)
	}
	if !transform.IsDigits(s) || len(s) != 6 {
		t.Fatalf("got %q", s)
	}
	empty, err := transform.RandomDigits(0)
	if err != nil || empty != "" {
		t.Fatalf("zero: %q %v", empty, err)
	}
	if _, err := transform.RandomDigits(-1); err == nil {
		t.Fatal("expected error for n<0")
	}
}

func TestRandomAlphaNumAndHex(t *testing.T) {
	s, err := transform.RandomAlphaNum(8)
	if err != nil || len(s) != 8 {
		t.Fatalf("alnum %q %v", s, err)
	}
	h, err := transform.RandomHex(4)
	if err != nil || len(h) != 8 {
		t.Fatalf("hex %q %v", h, err)
	}
}
