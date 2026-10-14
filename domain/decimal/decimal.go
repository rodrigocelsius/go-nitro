package decimal

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

// Decimal is an arbitrary-precision decimal stored as string canonically.
type Decimal struct {
	s string // canonical: optional sign, digits, optional .fraction
}

// NewFromString parses a decimal string.
func NewFromString(s string) (Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Decimal{s: "0"}, nil
	}
	r := new(big.Rat)
	if _, ok := r.SetString(s); !ok {
		return Decimal{}, fmt.Errorf("decimal: invalid %q", s)
	}
	return Decimal{s: ratString(r)}, nil
}

// NewFromFloat64 converts float64 (use for convenience; prefer string).
func NewFromFloat64(f float64) Decimal {
	r := new(big.Rat).SetFloat64(f)
	return Decimal{s: ratString(r)}
}

// NewFromInt64 creates an integer Decimal.
func NewFromInt64(n int64) Decimal {
	return Decimal{s: fmt.Sprintf("%d", n)}
}

func ratString(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String()
	}
	return r.FloatString(10)
}

// String returns canonical representation.
func (d Decimal) String() string {
	if d.s == "" {
		return "0"
	}
	return d.s
}

// IsZero reports whether d is 0.
func (d Decimal) IsZero() bool {
	r, ok := new(big.Rat).SetString(d.String())
	return ok && r.Sign() == 0
}

// Float64 approximates as float64.
func (d Decimal) Float64() (float64, bool) {
	r, ok := new(big.Rat).SetString(d.String())
	if !ok {
		return 0, false
	}
	f, _ := r.Float64()
	return f, true
}

// Value implements driver.Valuer.
func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}

// Scan implements sql.Scanner.
func (d *Decimal) Scan(value any) error {
	if d == nil {
		return fmt.Errorf("decimal: Scan on nil *Decimal")
	}
	if value == nil {
		*d = Decimal{s: "0"}
		return nil
	}
	switch v := value.(type) {
	case string:
		parsed, err := NewFromString(v)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case []byte:
		return d.Scan(string(v))
	case float64:
		*d = NewFromFloat64(v)
		return nil
	case int64:
		*d = NewFromInt64(v)
		return nil
	default:
		return fmt.Errorf("decimal: unsupported Scan type %T", value)
	}
}

func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Decimal) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*d = Decimal{s: "0"}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parsed, err := NewFromString(s)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	*d = NewFromFloat64(f)
	return nil
}
