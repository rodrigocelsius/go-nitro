package nitro_test

import (
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func TestTextHelpersFacade(t *testing.T) {
	if !nitro.IsDigits("123") || nitro.IsDigits("12a") {
		t.Fatal("IsDigits")
	}
	if nitro.Digits("(11) 9") != "119" {
		t.Fatal("Digits")
	}
	if !nitro.IsEmail("a@b.co") || nitro.IsEmail("nope") {
		t.Fatal("IsEmail")
	}
	if nitro.NormalizeEmail("  A@B.CO ") != "a@b.co" {
		t.Fatal("NormalizeEmail")
	}
	first, last := nitro.SplitName("João da Silva")
	if first != "João" || last != "da Silva" {
		t.Fatalf("SplitName %q %q", first, last)
	}
	code, err := nitro.RandomDigits(6)
	if err != nil || !nitro.IsDigits(code) || len(code) != 6 {
		t.Fatalf("RandomDigits %q %v", code, err)
	}
}

func TestPtrContains(t *testing.T) {
	p := nitro.Ptr(7)
	if nitro.Deref(p, 0) != 7 || nitro.Deref((*int)(nil), 3) != 3 {
		t.Fatal("Ptr/Deref")
	}
	if !nitro.Contains([]string{"admin", "driver"}, "admin") {
		t.Fatal("Contains")
	}
}
