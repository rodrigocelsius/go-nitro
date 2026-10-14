package transform

import "strings"

// IsDigits reports whether s is non-empty and consists only of ASCII digits (0-9).
func IsDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// SplitName splits a full name into first token and the remainder.
// Extra whitespace is collapsed. Empty input yields ("", "").
func SplitName(name string) (first, last string) {
	parts := strings.Fields(name)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], strings.Join(parts[1:], " ")
	}
}

// NormalizeEmail trims surrounding space and lowercases the address.
func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// IfBlank returns fallback when s is empty or only whitespace.
func IfBlank(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
