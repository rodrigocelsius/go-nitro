package temporal

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2/locale"
)

// FormatDate formats a Date for locale display.
func FormatDate(d Date, loc string) string {
	loc = locale.WithLocale(loc)
	if d.IsZero() {
		return ""
	}
	switch {
	case strings.HasPrefix(loc, "pt"), strings.HasPrefix(loc, "es"), strings.HasPrefix(loc, "fr"), strings.HasPrefix(loc, "de"):
		return fmt.Sprintf("%02d/%02d/%04d", d.Day(), d.Month(), d.Year())
	default:
		return fmt.Sprintf("%02d/%02d/%04d", d.Month(), d.Day(), d.Year())
	}
}

// FormatDateTime formats DateTime in the given locale and timezone.
func FormatDateTime(dt DateTime, loc string) string {
	loc = locale.WithLocale(loc)
	if dt.IsZero() {
		return ""
	}
	t := dt.Time().In(locale.Timezone())
	date := DateOf(t)
	tod := TimeOfDayOf(t)
	return FormatDate(date, loc) + ", " + fmt.Sprintf("%02d:%02d", tod.Hour(), tod.Minute())
}

// ParseDate parses a display date for locale into Date.
func ParseDate(s, loc string) (Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Date{}, ErrInvalidFormat
	}
	if d, err := ParseDateISO(s); err == nil {
		return d, nil
	}
	loc = locale.WithLocale(loc)
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '/' || r == '-' || r == '.'
	})
	if len(parts) != 3 {
		return Date{}, ErrInvalidFormat
	}
	a, err1 := strconv.Atoi(parts[0])
	b, err2 := strconv.Atoi(parts[1])
	c, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return Date{}, ErrInvalidFormat
	}
	var day, month, year int
	switch {
	case strings.HasPrefix(loc, "pt"), strings.HasPrefix(loc, "es"), strings.HasPrefix(loc, "fr"), strings.HasPrefix(loc, "de"):
		day, month, year = a, b, c
	default:
		month, day, year = a, b, c
	}
	if year < 100 {
		year += 2000
	}
	return NewDate(year, time.Month(month), day), nil
}

// ParseDateTime parses display or RFC3339 into DateTime (display uses locale.Timezone).
func ParseDateTime(s, loc string) (DateTime, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DateTime{}, ErrInvalidFormat
	}
	if dt, err := ParseDateTimeRFC3339(s); err == nil {
		return dt, nil
	}
	// "dd/MM/yyyy, HH:mm" or "dd/MM/yyyy HH:mm"
	s = strings.ReplaceAll(s, ",", " ")
	fields := strings.Fields(s)
	if len(fields) < 1 {
		return DateTime{}, ErrInvalidFormat
	}
	d, err := ParseDate(fields[0], loc)
	if err != nil {
		return DateTime{}, err
	}
	tod := TimeOfDay{}
	if len(fields) >= 2 {
		tod, err = ParseTimeOfDayISO(fields[1])
		if err != nil {
			return DateTime{}, err
		}
	}
	return CombineLocal(d, tod, locale.Timezone()), nil
}
