package money

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Money is a monetary amount in minor units (cents) + ISO 4217 currency.
type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// New creates Money from minor units.
func New(amount int64, currency string) Money {
	return Money{Amount: amount, Currency: strings.ToUpper(strings.TrimSpace(currency))}
}

// FromMajor converts a major-unit float (e.g. 10.50 BRL → 1050) with 2 decimals.
func FromMajor(major float64, currency string) Money {
	return New(int64(math.Round(major*100)), currency)
}

// Major returns amount / 100 as float64 (2-decimal currencies).
func (m Money) Major() float64 {
	return float64(m.Amount) / 100
}

// IsZero reports zero amount (currency may still be set).
func (m Money) IsZero() bool { return m.Amount == 0 }

// Equal compares amount and currency.
func (m Money) Equal(o Money) bool {
	return m.Amount == o.Amount && strings.EqualFold(m.Currency, o.Currency)
}

// String returns "amount currency" (e.g. "1050 BRL").
func (m Money) String() string {
	if m.Currency == "" {
		return strconv.FormatInt(m.Amount, 10)
	}
	return fmt.Sprintf("%d %s", m.Amount, m.Currency)
}

// Value implements driver.Valuer as JSON object (single-column fallback).
// Prefer expanded columns via migration codegen (*_amount + *_currency).
func (m Money) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan implements sql.Scanner from JSON or "amount|currency".
func (m *Money) Scan(value any) error {
	if m == nil {
		return fmt.Errorf("money: Scan on nil *Money")
	}
	if value == nil {
		*m = Money{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, m)
	case string:
		if strings.Contains(v, "|") {
			parts := strings.SplitN(v, "|", 2)
			n, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				return err
			}
			*m = New(n, parts[1])
			return nil
		}
		return json.Unmarshal([]byte(v), m)
	case int64:
		m.Amount = v
		return nil
	default:
		return fmt.Errorf("money: unsupported Scan type %T", value)
	}
}

// MarshalJSON encodes {"amount":…,"currency":"…"}.
func (m Money) MarshalJSON() ([]byte, error) {
	type alias Money
	return json.Marshal(alias(m))
}

// UnmarshalJSON accepts object or legacy major number with default currency unset.
func (m *Money) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*m = Money{}
		return nil
	}
	type alias Money
	var a alias
	if err := json.Unmarshal(data, &a); err == nil && (a.Currency != "" || a.Amount != 0 || string(data) == `{"amount":0,"currency":""}`) {
		*m = Money(a)
		m.Currency = strings.ToUpper(strings.TrimSpace(m.Currency))
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*m = FromMajor(n, "")
		return nil
	}
	return json.Unmarshal(data, (*alias)(m))
}

// FXProvider converts between currencies (stub for apps to implement).
type FXProvider interface {
	Rate(from, to string) (float64, error)
}

// ConvertCurrency converts m to toCurrency using p.
func ConvertCurrency(m Money, toCurrency string, p FXProvider) (Money, error) {
	if p == nil {
		return Money{}, fmt.Errorf("money: nil FXProvider")
	}
	toCurrency = strings.ToUpper(strings.TrimSpace(toCurrency))
	if strings.EqualFold(m.Currency, toCurrency) {
		return m, nil
	}
	rate, err := p.Rate(m.Currency, toCurrency)
	if err != nil {
		return Money{}, err
	}
	return New(int64(math.Round(float64(m.Amount)*rate)), toCurrency), nil
}
