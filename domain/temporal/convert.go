package temporal

import (
	"fmt"
	"time"
)

// CombineLocal joins a civil date and time-of-day in loc into a UTC DateTime.
func CombineLocal(d Date, tod TimeOfDay, loc *time.Location) DateTime {
	if loc == nil {
		loc = time.UTC
	}
	t := time.Date(d.Year(), d.Month(), d.Day(), tod.Hour(), tod.Minute(), tod.Second(), tod.Nanosecond(), loc)
	return DateTimeFrom(t)
}

// SplitLocal splits a DateTime into civil date and time-of-day in loc.
func SplitLocal(dt DateTime, loc *time.Location) (Date, TimeOfDay) {
	if loc == nil {
		loc = time.UTC
	}
	t := dt.Time().In(loc)
	return DateOf(t), TimeOfDayOf(t)
}

// ParseDateTimeRFC3339 parses an RFC3339 instant into DateTime.
func ParseDateTimeRFC3339(s string) (DateTime, error) {
	var d DateTime
	if err := d.parseString(s); err != nil {
		return DateTime{}, err
	}
	return d, nil
}

// MustParseDateISO panics on error (tests / static data).
func MustParseDateISO(s string) Date {
	d, err := ParseDateISO(s)
	if err != nil {
		panic(err)
	}
	return d
}

// MustParseTimeOfDayISO panics on error.
func MustParseTimeOfDayISO(s string) TimeOfDay {
	t, err := ParseTimeOfDayISO(s)
	if err != nil {
		panic(err)
	}
	return t
}

// ErrInvalidFormat is returned by locale-aware parsers on bad input.
var ErrInvalidFormat = fmt.Errorf("temporal: invalid format")
