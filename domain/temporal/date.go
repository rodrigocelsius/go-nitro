package temporal

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Date is a civil calendar day without timezone (JSON: "YYYY-MM-DD").
type Date struct {
	year  int
	month time.Month
	day   int
}

// NewDate builds a Date (month/day validated via time.Date).
func NewDate(year int, month time.Month, day int) Date {
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return Date{year: t.Year(), month: t.Month(), day: t.Day()}
}

// DateOf extracts the civil date from a time in that location's calendar.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{year: y, month: m, day: d}
}

// ParseDateISO parses "YYYY-MM-DD".
func ParseDateISO(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, fmt.Errorf("date: parse %q: %w", s, err)
	}
	return DateOf(t), nil
}

// Year / Month / Day accessors.
func (d Date) Year() int         { return d.year }
func (d Date) Month() time.Month { return d.month }
func (d Date) Day() int          { return d.day }

// IsZero reports whether d is unset.
func (d Date) IsZero() bool {
	return d.year == 0 && d.month == 0 && d.day == 0
}

// String returns ISO "YYYY-MM-DD".
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.year, d.month, d.day)
}

// Time returns midnight UTC for this civil date.
func (d Date) Time() time.Time {
	if d.IsZero() {
		return time.Time{}
	}
	return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, time.UTC)
}

// Equal compares civil dates.
func (d Date) Equal(o Date) bool {
	return d.year == o.year && d.month == o.month && d.day == o.day
}

// Value implements driver.Valuer.
func (d Date) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.String(), nil
}

// Scan implements sql.Scanner.
func (d *Date) Scan(value any) error {
	if d == nil {
		return fmt.Errorf("date: Scan on nil *Date")
	}
	if value == nil {
		*d = Date{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*d = DateOf(v)
		return nil
	case string:
		if v == "" {
			*d = Date{}
			return nil
		}
		parsed, err := ParseDateISO(v[:min(len(v), 10)])
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case []byte:
		return d.Scan(string(v))
	default:
		return fmt.Errorf("date: unsupported Scan type %T", value)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MarshalJSON encodes as "YYYY-MM-DD" or null.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

// UnmarshalJSON accepts "YYYY-MM-DD" or null.
func (d *Date) UnmarshalJSON(data []byte) error {
	if d == nil {
		return fmt.Errorf("date: UnmarshalJSON on nil *Date")
	}
	if string(data) == "null" || string(data) == `""` {
		*d = Date{}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*d = Date{}
		return nil
	}
	parsed, err := ParseDateISO(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
