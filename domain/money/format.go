package money

import (
	"fmt"
	"strconv"
	"strings"
)

// Format returns a locale-ish display string (pt-BR / en-US style).
// Uses 2 decimal places for major units.
func Format(m Money, locale string) string {
	major := m.Major()
	neg := major < 0
	if neg {
		major = -major
	}
	intPart := int64(major)
	frac := int64(mathRound((major - float64(intPart)) * 100))
	if frac >= 100 {
		intPart++
		frac = 0
	}
	intStr := formatIntGrouped(intPart, locale)
	var s string
	switch normalizeLocale(locale) {
	case "pt-BR", "pt-PT", "de-DE", "es-ES", "fr-FR":
		s = fmt.Sprintf("%s,%02d", intStr, frac)
	default:
		s = fmt.Sprintf("%s.%02d", intStr, frac)
	}
	if neg {
		s = "-" + s
	}
	if m.Currency != "" {
		return s + " " + m.Currency
	}
	return s
}

// Parse parses a display money string for locale into Money (2 decimals).
// Accepts "1.234,56 BRL", "1,234.56 USD", "1234.56", etc.
func Parse(s, currency, locale string) (Money, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Money{}, fmt.Errorf("money: empty")
	}
	parts := strings.Fields(s)
	num := parts[0]
	cur := currency
	if len(parts) > 1 && cur == "" {
		cur = parts[len(parts)-1]
		num = strings.Join(parts[:len(parts)-1], "")
	}
	num = strings.TrimSpace(num)
	neg := strings.HasPrefix(num, "-")
	num = strings.TrimPrefix(num, "-")
	num = strings.TrimPrefix(num, "+")

	var major float64
	var err error
	switch normalizeLocale(locale) {
	case "pt-BR", "pt-PT", "de-DE", "es-ES", "fr-FR":
		num = strings.ReplaceAll(num, ".", "")
		num = strings.ReplaceAll(num, ",", ".")
		major, err = strconv.ParseFloat(num, 64)
	default:
		num = strings.ReplaceAll(num, ",", "")
		major, err = strconv.ParseFloat(num, 64)
	}
	if err != nil {
		return Money{}, fmt.Errorf("money: parse %q: %w", s, err)
	}
	if neg {
		major = -major
	}
	return FromMajor(major, cur), nil
}

func normalizeLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return "en-US"
	}
	locale = strings.ReplaceAll(locale, "_", "-")
	return locale
}

func formatIntGrouped(n int64, locale string) string {
	s := strconv.FormatInt(n, 10)
	var sep byte
	switch normalizeLocale(locale) {
	case "pt-BR", "pt-PT", "de-DE", "es-ES", "fr-FR":
		sep = '.'
	default:
		sep = ','
	}
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
		s = s[rem:]
		if len(s) > 0 {
			b.WriteByte(sep)
		}
	}
	for len(s) > 3 {
		b.WriteString(s[:3])
		b.WriteByte(sep)
		s = s[3:]
	}
	b.WriteString(s)
	return b.String()
}

func mathRound(x float64) float64 {
	if x < 0 {
		return float64(int64(x - 0.5))
	}
	return float64(int64(x + 0.5))
}
