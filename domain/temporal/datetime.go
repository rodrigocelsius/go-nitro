package temporal

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DateTime is a UTC instant (replaces time.Time in Nitro models).
// JSON wire format is RFC3339 with Z.
type DateTime struct {
	t time.Time
}

// Now returns the current UTC instant.
func Now() DateTime {
	return DateTime{t: time.Now().UTC()}
}

// DateTimeFrom converts any time.Time to UTC DateTime.
func DateTimeFrom(t time.Time) DateTime {
	return DateTime{t: t.UTC()}
}

// Time returns the underlying UTC time.Time.
func (d DateTime) Time() time.Time { return d.t }

// AsTime is an alias of Time for clarity at call sites.
func (d DateTime) AsTime() time.Time { return d.t }

// IsZero reports whether d is the zero DateTime.
func (d DateTime) IsZero() bool { return d.t.IsZero() }

// Equal reports whether d and o represent the same instant.
func (d DateTime) Equal(o DateTime) bool { return d.t.Equal(o.t) }

// After reports whether d is after o.
func (d DateTime) After(o DateTime) bool { return d.t.After(o.t) }

// Before reports whether d is before o.
func (d DateTime) Before(o DateTime) bool { return d.t.Before(o.t) }

// Add returns d + duration.
func (d DateTime) Add(dur time.Duration) DateTime {
	return DateTime{t: d.t.Add(dur)}
}

// FormatRFC3339 returns canonical wire format.
func (d DateTime) FormatRFC3339() string {
	if d.IsZero() {
		return ""
	}
	return d.t.UTC().Format(time.RFC3339Nano)
}

// Value implements driver.Valuer.
// Returns time.Time so MySQL/MariaDB DATETIME and SQL Server DATETIMEOFFSET accept the value;
// SQLite drivers stringify it (Scan accepts RFC3339 and Go time.Time.String layouts).
func (d DateTime) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.t.UTC(), nil
}

// Scan implements sql.Scanner.
func (d *DateTime) Scan(value any) error {
	if d == nil {
		return fmt.Errorf("datetime: Scan on nil *DateTime")
	}
	if value == nil {
		*d = DateTime{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*d = DateTimeFrom(v)
		return nil
	case string:
		return d.parseString(v)
	case []byte:
		return d.parseString(string(v))
	default:
		return fmt.Errorf("datetime: unsupported Scan type %T", value)
	}
}

func (d *DateTime) parseString(s string) error {
	if s == "" {
		*d = DateTime{}
		return nil
	}
	// Pick the likely layout from the shape first: every failed time.Parse
	// allocates an error, and scanning N rows × M timestamp columns through a
	// fixed list (SQLite returns time.Time.String() text) dominated profiles.
	for _, layout := range dateTimeLayoutsFor(s) {
		if t, err := time.Parse(layout, s); err == nil {
			*d = DateTimeFrom(t)
			return nil
		}
	}
	return fmt.Errorf("datetime: cannot parse %q", s)
}

// MarshalJSON encodes as RFC3339 string (or null if zero).
func (d DateTime) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	// RFC 3339 text never needs JSON escaping: one allocation.
	b := make([]byte, 0, len(time.RFC3339Nano)+2)
	b = append(b, '"')
	b = d.t.UTC().AppendFormat(b, time.RFC3339Nano)
	return append(b, '"'), nil
}

// UnmarshalJSON accepts RFC3339 / null.
func (d *DateTime) UnmarshalJSON(data []byte) error {
	if d == nil {
		return fmt.Errorf("datetime: UnmarshalJSON on nil *DateTime")
	}
	if string(data) == "null" || string(data) == `""` {
		*d = DateTime{}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return d.parseString(s)
}

var (
	// "2006-01-02T…": RFC 3339 first.
	dateTimeLayoutsT = []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999"}
	// "2006-01-02 … -0700 MST": time.Time.String() (SQLite drivers).
	dateTimeLayoutsZone = []string{"2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05.999999999 -0700"}
	// "2006-01-02 …±07:00" or naive "2006-01-02 15:04:05[.fff]".
	dateTimeLayoutsSpace = []string{"2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999"}
	dateTimeLayoutsAll   = []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999",
	}
)

func dateTimeLayoutsFor(s string) []string {
	if len(s) < 19 {
		return dateTimeLayoutsAll
	}
	switch s[10] {
	case 'T':
		return dateTimeLayoutsT
	case ' ':
		if strings.IndexByte(s[19:], ' ') >= 0 {
			return dateTimeLayoutsZone
		}
		return dateTimeLayoutsSpace
	}
	return dateTimeLayoutsAll
}
