package measure

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatMeasure formats for display (e.g. "10.94 yd").
func FormatMeasure(m Measure, locale string) string {
	prec := 2
	s := strconv.FormatFloat(m.Val, 'f', prec, 64)
	if strings.HasPrefix(normalize(locale), "pt") || strings.HasPrefix(normalize(locale), "de") {
		s = strings.ReplaceAll(s, ".", ",")
	}
	return s + " " + string(m.Unit)
}

// ParseMeasure parses "10 yards", "10.5 ft", "10 m".
func ParseMeasure(s, locale string) (Measure, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Measure{}, fmt.Errorf("measure: empty")
	}
	parts := strings.Fields(s)
	if len(parts) < 1 {
		return Measure{}, fmt.Errorf("measure: invalid %q", s)
	}
	numStr := parts[0]
	if strings.HasPrefix(normalize(locale), "pt") || strings.HasPrefix(normalize(locale), "de") {
		numStr = strings.ReplaceAll(numStr, ".", "")
		numStr = strings.ReplaceAll(numStr, ",", ".")
	} else {
		numStr = strings.ReplaceAll(numStr, ",", "")
	}
	v, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return Measure{}, fmt.Errorf("measure: parse number in %q: %w", s, err)
	}
	unit := Meter
	if len(parts) >= 2 {
		u := strings.ToLower(parts[1])
		u = strings.TrimSuffix(u, "s") // yards → yard
		switch u {
		case "m", "meter", "metre":
			unit = Meter
		case "cm", "centimeter", "centimetre":
			unit = Centimeter
		case "km", "kilometer", "kilometre":
			unit = Kilometer
		case "in", "inch":
			unit = Inch
		case "ft", "foot", "feet":
			unit = Foot
		case "yd", "yard":
			unit = Yard
		case "mi", "mile":
			unit = Mile
		case "kg", "kilogram":
			unit = Kilogram
		case "g", "gram":
			unit = Gram
		case "lb", "pound":
			unit = Pound
		case "oz", "ounce":
			unit = Ounce
		case "c", "celsius":
			unit = Celsius
		case "f", "fahrenheit":
			unit = Fahrenheit
		case "k", "kelvin":
			unit = Kelvin
		case "l", "liter", "litre":
			unit = Liter
		case "gal", "gallon":
			unit = GallonUS
		default:
			unit = Unit(parts[1])
		}
	}
	return New(v, unit), nil
}

func normalize(locale string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(locale)), "_", "-")
}
