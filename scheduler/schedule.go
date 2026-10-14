package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseSchedule parses a schedule string and returns a Schedule.
// Supported formats:
//   - "5m" - every 5 minutes
//   - "1h" - every hour
//   - "24h" - every 24 hours
//   - "*/5 * * * *" - cron expression (minute, hour, day of month, month, day of week)
func ParseSchedule(s string) (Schedule, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("scheduler: empty schedule")
	}

	// Try parsing as an interval
	if interval, err := parseInterval(s); err == nil {
		return &IntervalSchedule{Interval: interval}, nil
	}

	// Try parsing as cron
	if cron, err := parseCron(s); err == nil {
		return cron, nil
	}

	return nil, fmt.Errorf("scheduler: invalid schedule format: %q", s)
}

// parseInterval parses strings such as "5m", "1h", "24h", etc.
func parseInterval(s string) (time.Duration, error) {
	s = strings.ToLower(s)

	// Simple format: number + unit
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid interval")
	}

	numStr := s[:len(s)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid interval number")
	}

	if num <= 0 {
		return 0, fmt.Errorf("interval must be positive")
	}

	unit := s[len(s)-1]
	switch unit {
	case 's':
		return time.Duration(num) * time.Second, nil
	case 'm':
		return time.Duration(num) * time.Minute, nil
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid interval unit: %c", unit)
	}
}

// IntervalSchedule represents a fixed-interval schedule.
type IntervalSchedule struct {
	Interval time.Duration
}

func (s *IntervalSchedule) Next(after time.Time) time.Time {
	return after.Add(s.Interval)
}

func (s *IntervalSchedule) String() string {
	return fmt.Sprintf("every %s", s.Interval)
}

// CronSchedule represents a cron-expression schedule.
type CronSchedule struct {
	Minute     cronField
	Hour       cronField
	DayOfMonth cronField
	Month      cronField
	DayOfWeek  cronField
}

func (s *CronSchedule) Next(after time.Time) time.Time {
	t := after.Add(time.Minute)
	t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())

	for i := 0; i < 366*24*60; i++ { // Limita a 1 ano
		if s.matches(t) {
			return t
		}
		t = t.Add(time.Minute)
	}

	return after.Add(24 * time.Hour) // Fallback
}

func (s *CronSchedule) matches(t time.Time) bool {
	return s.Minute.matches(t.Minute()) &&
		s.Hour.matches(t.Hour()) &&
		s.DayOfMonth.matches(t.Day()) &&
		s.Month.matches(int(t.Month())) &&
		s.DayOfWeek.matches(int(t.Weekday()))
}

func (s *CronSchedule) String() string {
	return fmt.Sprintf("cron: %s %s %s %s %s",
		s.Minute, s.Hour, s.DayOfMonth, s.Month, s.DayOfWeek)
}

// cronField represents a cron-expression field.
type cronField struct {
	Type  string // "any", "specific", "range", "step"
	Value int
	Min   int
	Max   int
	Step  int
}

func (f *cronField) matches(v int) bool {
	switch f.Type {
	case "any":
		return true
	case "specific":
		return v == f.Value
	case "range":
		return v >= f.Min && v <= f.Max
	case "step":
		return v%f.Step == 0
	default:
		return false
	}
}

func (f cronField) String() string {
	switch f.Type {
	case "any":
		return "*"
	case "specific":
		return strconv.Itoa(f.Value)
	case "range":
		return fmt.Sprintf("%d-%d", f.Min, f.Max)
	case "step":
		return fmt.Sprintf("*/%d", f.Step)
	default:
		return "*"
	}
}

// parseCron parses a 5-field cron expression.
func parseCron(s string) (*CronSchedule, error) {
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields")
	}

	schedule := &CronSchedule{}

	var err error
	if schedule.Minute, err = parseCronField(fields[0], 0, 59); err != nil {
		return nil, fmt.Errorf("invalid minute field: %w", err)
	}

	if schedule.Hour, err = parseCronField(fields[1], 0, 23); err != nil {
		return nil, fmt.Errorf("invalid hour field: %w", err)
	}

	if schedule.DayOfMonth, err = parseCronField(fields[2], 1, 31); err != nil {
		return nil, fmt.Errorf("invalid day of month field: %w", err)
	}

	if schedule.Month, err = parseCronField(fields[3], 1, 12); err != nil {
		return nil, fmt.Errorf("invalid month field: %w", err)
	}

	if schedule.DayOfWeek, err = parseCronField(fields[4], 0, 6); err != nil {
		return nil, fmt.Errorf("invalid day of week field: %w", err)
	}

	return schedule, nil
}

// parseCronField parses a cron-expression field.
func parseCronField(s string, min, max int) (cronField, error) {
	if s == "*" {
		return cronField{Type: "any"}, nil
	}

	// Step: */n
	if strings.HasPrefix(s, "*/") {
		stepStr := s[2:]
		step, err := strconv.Atoi(stepStr)
		if err != nil || step <= 0 {
			return cronField{}, fmt.Errorf("invalid step: %s", s)
		}
		return cronField{Type: "step", Step: step}, nil
	}

	// Range: n-m
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		minVal, err := strconv.Atoi(parts[0])
		if err != nil {
			return cronField{}, fmt.Errorf("invalid range start: %s", parts[0])
		}
		maxVal, err := strconv.Atoi(parts[1])
		if err != nil {
			return cronField{}, fmt.Errorf("invalid range end: %s", parts[1])
		}
		if minVal < min || maxVal > max || minVal > maxVal {
			return cronField{}, fmt.Errorf("invalid range: %s", s)
		}
		return cronField{Type: "range", Min: minVal, Max: maxVal}, nil
	}

	// Specific value
	val, err := strconv.Atoi(s)
	if err != nil {
		return cronField{}, fmt.Errorf("invalid value: %s", s)
	}
	if val < min || val > max {
		return cronField{}, fmt.Errorf("value out of range: %s", s)
	}
	return cronField{Type: "specific", Value: val}, nil
}
