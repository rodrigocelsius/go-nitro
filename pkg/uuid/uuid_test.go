package uuid

import (
	"strings"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	u := New()
	if u.IsZero() {
		t.Fatal("expected non-zero UUID")
	}
}

func TestString(t *testing.T) {
	u := New()
	s := u.String()
	if len(s) != 36 {
		t.Fatalf("expected length 36, got %d", len(s))
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		t.Fatalf("invalid format: %s", s)
	}
}

func TestParse(t *testing.T) {
	original := New()
	parsed, err := Parse(original.String())
	if err != nil {
		t.Fatal(err)
	}
	if original != parsed {
		t.Fatalf("mismatch: %s != %s", original, parsed)
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"too short", "abc"},
		{"too long", "0123456789abcdef0123456789abcdef012"},
		{"bad dash", "01234567-89ab-cdef-0123-456789abcde"},
		{"invalid hex", "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestVersion(t *testing.T) {
	u := New()
	if u[6]>>4 != 7 {
		t.Fatalf("expected version 7, got %d", u[6]>>4)
	}
}

func TestVariant(t *testing.T) {
	u := New()
	if u[8]&0xc0 != 0x80 {
		t.Fatalf("expected variant 10xx, got %08b", u[8])
	}
}

func TestMonotonicity(t *testing.T) {
	var prev UUID
	for i := 0; i < 1000; i++ {
		u := New()
		if u.String() <= prev.String() && i > 0 {
			t.Fatalf("not monotonic at %d: %s <= %s", i, u, prev)
		}
		prev = u
	}
}

func TestFromBytes(t *testing.T) {
	original := New()
	b := original.Bytes()
	parsed, err := FromBytes(b)
	if err != nil {
		t.Fatal(err)
	}
	if original != parsed {
		t.Fatal("mismatch")
	}
}

func TestFromBytesInvalid(t *testing.T) {
	_, err := FromBytes([]byte{1, 2, 3})
	if err != ErrInvalidLength {
		t.Fatal("expected ErrInvalidLength")
	}
}

func TestIsZero(t *testing.T) {
	var u UUID
	if !u.IsZero() {
		t.Fatal("expected zero UUID")
	}
	u = New()
	if u.IsZero() {
		t.Fatal("expected non-zero UUID")
	}
}

func TestNewString(t *testing.T) {
	s := NewString()
	if len(s) != 36 {
		t.Fatalf("expected length 36, got %d", len(s))
	}
}

func TestNewBytes(t *testing.T) {
	b := NewBytes()
	if len(b) != Size {
		t.Fatalf("expected length %d, got %d", Size, len(b))
	}
}

func TestUniqueness(t *testing.T) {
	seen := make(map[UUID]bool)
	for i := 0; i < 10000; i++ {
		u := New()
		if seen[u] {
			t.Fatalf("duplicate at %d", i)
		}
		seen[u] = true
	}
}

func TestTimestampOrdering(t *testing.T) {
	u1 := New()
	u2 := New()
	if u1.String() >= u2.String() {
		t.Fatal("UUIDs should be time-ordered")
	}
}

func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New()
	}
}

func BenchmarkString(b *testing.B) {
	u := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = u.String()
	}
}

func BenchmarkParse(b *testing.B) {
	s := New().String()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(s)
	}
}

func BenchmarkNewString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewString()
	}
}

func BenchmarkNewBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewBytes()
	}
}

func BenchmarkFromBytes(b *testing.B) {
	bb := New().Bytes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FromBytes(bb)
	}
}

func TestConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	uuids := make(chan UUID, 100)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				uuids <- New()
			}
		}()
	}
	go func() {
		wg.Wait()
		close(uuids)
	}()

	seen := make(map[UUID]bool)
	for u := range uuids {
		if seen[u] {
			t.Fatal("concurrent duplicate")
		}
		seen[u] = true
	}
	if len(seen) != 10000 {
		t.Fatalf("expected 10000, got %d", len(seen))
	}
}

func TestParseEdgeCases(t *testing.T) {
	s := "01234567-89ab-cdef-0123-456789abcdef"
	u, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	if u.String() != strings.ToLower(s) {
		t.Fatalf("mismatch: %s != %s", u.String(), s)
	}

	_, err = Parse("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}

	_, err = Parse("01234567-89ab-cdef-0123-456789abcdeg")
	if err != ErrInvalidFormat {
		t.Fatal("expected ErrInvalidFormat")
	}
}

func TestHexChar(t *testing.T) {
	for c := byte(0); c < 255; c++ {
		v := fromHexChar(c)
		if v == 0xff {
			continue
		}
		if c >= '0' && c <= '9' {
			if v != c-'0' {
				t.Fatal("invalid digit")
			}
		} else if c >= 'a' && c <= 'f' {
			if v != c-'a'+10 {
				t.Fatal("invalid lowercase")
			}
		} else if c >= 'A' && c <= 'F' {
			if v != c-'A'+10 {
				t.Fatal("invalid uppercase")
			}
		} else {
			t.Fatalf("unexpected valid char: %c", c)
		}
	}
}
