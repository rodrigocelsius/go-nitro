package money

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
)

// ErrCurrencyMismatch is returned when amounts in different currencies meet.
var ErrCurrencyMismatch = errors.New("money: currency mismatch")

// ErrOverflow is returned when a result does not fit in int64 minor units.
var ErrOverflow = errors.New("money: overflow")

func same(a, b Money) error {
	if !strings.EqualFold(a.Currency, b.Currency) && !a.IsZero() && !b.IsZero() {
		return fmt.Errorf("%w: %s and %s", ErrCurrencyMismatch, a.Currency, b.Currency)
	}
	return nil
}

func currencyOf(a, b Money) string {
	if a.Currency != "" {
		return a.Currency
	}
	return b.Currency
}

// Add returns m + o. A zero amount takes the other's currency.
func (m Money) Add(o Money) (Money, error) {
	if err := same(m, o); err != nil {
		return Money{}, err
	}
	s := m.Amount + o.Amount
	if (o.Amount > 0 && s < m.Amount) || (o.Amount < 0 && s > m.Amount) {
		return Money{}, ErrOverflow
	}
	return Money{Amount: s, Currency: currencyOf(m, o)}, nil
}

// Sub returns m - o.
func (m Money) Sub(o Money) (Money, error) {
	if o.Amount == math.MinInt64 {
		return Money{}, ErrOverflow
	}
	return m.Add(Money{Amount: -o.Amount, Currency: o.Currency})
}

// Times returns m × n.
func (m Money) Times(n int64) (Money, error) {
	p := new(big.Int).Mul(big.NewInt(m.Amount), big.NewInt(n))
	if !p.IsInt64() {
		return Money{}, ErrOverflow
	}
	return Money{Amount: p.Int64(), Currency: m.Currency}, nil
}

// Ratio returns m × num / den, rounded half away from zero, computed exactly
// (pro-rata: Ratio(daysLeft, daysInPeriod); usage: Ratio(bytes, bytesPerUnit)).
func (m Money) Ratio(num, den int64) (Money, error) {
	if den == 0 {
		return Money{}, errors.New("money: ratio with zero denominator")
	}
	p := new(big.Int).Mul(big.NewInt(m.Amount), big.NewInt(num))
	d := big.NewInt(den)
	if d.Sign() < 0 {
		p.Neg(p)
		d.Neg(d)
	}
	q, r := new(big.Int).QuoRem(p, d, new(big.Int))
	// |2r| >= d rounds away from zero.
	if new(big.Int).Abs(new(big.Int).Lsh(r, 1)).Cmp(d) >= 0 {
		if p.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	if !q.IsInt64() {
		return Money{}, ErrOverflow
	}
	return Money{Amount: q.Int64(), Currency: m.Currency}, nil
}

// Neg returns -m.
func (m Money) Neg() Money { return Money{Amount: -m.Amount, Currency: m.Currency} }

// IsNegative reports m < 0.
func (m Money) IsNegative() bool { return m.Amount < 0 }

// Cmp compares two amounts of the same currency: -1, 0 or +1.
func (m Money) Cmp(o Money) (int, error) {
	if err := same(m, o); err != nil {
		return 0, err
	}
	switch {
	case m.Amount < o.Amount:
		return -1, nil
	case m.Amount > o.Amount:
		return 1, nil
	}
	return 0, nil
}

// Sum adds amounts of one currency; the empty sum is zero in currency.
func Sum(currency string, ms ...Money) (Money, error) {
	total := New(0, currency)
	for _, x := range ms {
		var err error
		if total, err = total.Add(x); err != nil {
			return Money{}, err
		}
	}
	return total, nil
}

// Allocate splits m by weights without losing a cent: the remainder goes,
// one minor unit each, to the first shares (invoice lines, split payments).
func (m Money) Allocate(weights ...int64) ([]Money, error) {
	var total int64
	for _, w := range weights {
		if w < 0 {
			return nil, errors.New("money: negative weight")
		}
		total += w
	}
	if total == 0 {
		return nil, errors.New("money: weights sum to zero")
	}
	out := make([]Money, len(weights))
	var given int64
	for i, w := range weights {
		p := new(big.Int).Mul(big.NewInt(m.Amount), big.NewInt(w))
		p.Quo(p, big.NewInt(total))
		out[i] = Money{Amount: p.Int64(), Currency: m.Currency}
		given += out[i].Amount
	}
	step := int64(1)
	if m.Amount < 0 {
		step = -1
	}
	for i := 0; given != m.Amount; i = (i + 1) % len(out) {
		if weights[i] == 0 {
			continue
		}
		out[i].Amount += step
		given += step
	}
	return out, nil
}
