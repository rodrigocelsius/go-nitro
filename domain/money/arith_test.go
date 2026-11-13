package money

import (
	"errors"
	"math"
	"testing"
)

func TestArithmetic(t *testing.T) {
	a, b := New(1050, "brl"), New(250, "BRL")
	if s, _ := a.Add(b); s.Amount != 1300 || s.Currency != "BRL" {
		t.Fatal(s)
	}
	if d, _ := b.Sub(a); d.Amount != -800 || !d.IsNegative() {
		t.Fatal(d)
	}
	if _, err := a.Add(New(1, "USD")); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatal(err)
	}
	if s, _ := New(0, "").Add(a); s.Currency != "BRL" {
		t.Fatal("zero takes the other's currency")
	}
	if _, err := New(math.MaxInt64, "BRL").Add(New(1, "BRL")); !errors.Is(err, ErrOverflow) {
		t.Fatal(err)
	}
	if x, _ := a.Times(3); x.Amount != 3150 {
		t.Fatal(x)
	}
	if _, err := New(math.MaxInt64/2+1, "BRL").Times(2); !errors.Is(err, ErrOverflow) {
		t.Fatal(err)
	}
	if s, _ := Sum("BRL", a, b, b); s.Amount != 1550 {
		t.Fatal(s)
	}
	if c, _ := a.Cmp(b); c != 1 {
		t.Fatal(c)
	}
}

func TestRatioRoundsHalfAwayFromZero(t *testing.T) {
	cases := []struct{ amount, num, den, want int64 }{
		{2990, 15, 30, 1495},
		{2990, 1, 3, 997}, // 996.67
		{1000, 1, 8, 125}, // exact
		{5, 1, 2, 3},      // 2.5 → 3
		{-5, 1, 2, -3},    // -2.5 → -3
		{1, 1, 3, 0},
		{math.MaxInt64, 3, 4, 6917529027641081855}, // exact big arithmetic
	}
	for _, c := range cases {
		got, err := New(c.amount, "BRL").Ratio(c.num, c.den)
		if err != nil || got.Amount != c.want {
			t.Errorf("%d×%d/%d = %v %v, want %d", c.amount, c.num, c.den, got.Amount, err, c.want)
		}
	}
	if _, err := New(1, "BRL").Ratio(1, 0); err == nil {
		t.Fatal("zero denominator")
	}
}

func TestAllocateKeepsEveryCent(t *testing.T) {
	parts, err := New(1000, "BRL").Allocate(1, 1, 1)
	if err != nil || parts[0].Amount != 334 || parts[1].Amount != 333 || parts[2].Amount != 333 {
		t.Fatal(parts, err)
	}
	parts, _ = New(-100, "BRL").Allocate(1, 0, 2)
	if parts[0].Amount+parts[1].Amount+parts[2].Amount != -100 || parts[1].Amount != 0 {
		t.Fatal(parts)
	}
	if _, err := New(1, "BRL").Allocate(0, 0); err == nil {
		t.Fatal("zero weights")
	}
}
