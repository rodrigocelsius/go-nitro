package temporal

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TimeOfDay is a clock time without date/timezone (JSON: "HH:mm:ss").
type TimeOfDay struct {
	hour, min, sec, nsec int
}

// NewTimeOfDay builds a TimeOfDay (values normalized via time.Duration).
func NewTimeOfDay(hour, min, sec, nsec int) TimeOfDay {
	d := time.Duration(hour)*time.Hour +
		time.Duration(min)*time.Minute +
		time.Duration(sec)*time.Second +
		time.Duration(nsec)*time.Nanosecond
	t := time.Unix(0, 0).UTC().Add(d)
	return TimeOfDay{hour: t.Hour(), min: t.Minute(), sec: t.Second(), nsec: t.Nanosecond()}
}

// TimeOfDayOf extracts clock components from t.
func TimeOfDayOf(t time.Time) TimeOfDay {
	return TimeOfDay{hour: t.Hour(), min: t.Minute(), sec: t.Second(), nsec: t.Nanosecond()}
}

// ParseTimeOfDayISO parses "HH:mm", "HH:mm:ss", or "HH:mm:ss.nnnnnnnnn".
func ParseTimeOfDayISO(s string) (TimeOfDay, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return TimeOfDay{}, nil
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return TimeOfDay{}, fmt.Errorf("timeofday: invalid %q", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return TimeOfDay{}, fmt.Errorf("timeofday: invalid hour in %q", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return TimeOfDay{}, fmt.Errorf("timeofday: invalid minute in %q", s)
	}
	sec, nsec := 0, 0
	if len(parts) == 3 {
		sp := strings.SplitN(parts[2], ".", 2)
		sec, err = strconv.Atoi(sp[0])
		if err != nil || sec < 0 || sec > 59 {
			return TimeOfDay{}, fmt.Errorf("timeofday: invalid second in %q", s)
		}
		if len(sp) == 2 {
			frac := sp[1]
			if len(frac) > 9 {
				frac = frac[:9]
			}
			for len(frac) < 9 {
				frac += "0"
			}
			nsec, err = strconv.Atoi(frac)
			if err != nil {
				return TimeOfDay{}, fmt.Errorf("timeofday: invalid fraction in %q", s)
			}
		}
	}
	return TimeOfDay{hour: h, min: m, sec: sec, nsec: nsec}, nil
}

func (t TimeOfDay) Hour() int       { return t.hour }
func (t TimeOfDay) Minute() int     { return t.min }
func (t TimeOfDay) Second() int     { return t.sec }
func (t TimeOfDay) Nanosecond() int { return t.nsec }

// IsZero reports whether t is midnight with zero nanos AND all zero — use
// explicitly constructed NewTimeOfDay(0,0,0,0) which is also "zero-looking".
// Convention: unset TimeOfDay is the zero value; midnight is valid and equal.
func (t TimeOfDay) IsZero() bool {
	return t.hour == 0 && t.min == 0 && t.sec == 0 && t.nsec == 0
}

// String returns "HH:mm:ss" (fraction omitted when zero).
func (t TimeOfDay) String() string {
	if t.nsec == 0 {
		return fmt.Sprintf("%02d:%02d:%02d", t.hour, t.min, t.sec)
	}
	return fmt.Sprintf("%02d:%02d:%02d.%09d", t.hour, t.min, t.sec, t.nsec)
}

// Duration since midnight.
func (t TimeOfDay) Duration() time.Duration {
	return time.Duration(t.hour)*time.Hour +
		time.Duration(t.min)*time.Minute +
		time.Duration(t.sec)*time.Second +
		time.Duration(t.nsec)*time.Nanosecond
}

// Value implements driver.Valuer.
func (t TimeOfDay) Value() (driver.Value, error) {
	if t.IsZero() {
		return "", nil
	}
	return t.String(), nil
}

// Scan implements sql.Scanner.
func (t *TimeOfDay) Scan(value any) error {
	if t == nil {
		return fmt.Errorf("timeofday: Scan on nil *TimeOfDay")
	}
	if value == nil {
		*t = TimeOfDay{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*t = TimeOfDayOf(v)
		return nil
	case string:
		parsed, err := ParseTimeOfDayISO(v)
		if err != nil {
			return err
		}
		*t = parsed
		return nil
	case []byte:
		return t.Scan(string(v))
	default:
		return fmt.Errorf("timeofday: unsupported Scan type %T", value)
	}
}

// MarshalJSON encodes as "HH:mm:ss".
func (t TimeOfDay) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// UnmarshalJSON accepts "HH:mm[:ss]" or null.
func (t *TimeOfDay) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("timeofday: UnmarshalJSON on nil *TimeOfDay")
	}
	if string(data) == "null" || string(data) == `""` {
		*t = TimeOfDay{}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseTimeOfDayISO(s)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}
