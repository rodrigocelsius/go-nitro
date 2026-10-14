package validator

import "strings"

// IsEmail reports whether s looks like an email (same rules as validate:"email").
// Surrounding space is ignored. Empty string is invalid.
func IsEmail(s string) bool {
	email := strings.TrimSpace(s)
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	local := email[:at]
	domainPart := email[at+1:]
	if local == "" || strings.Contains(local, " ") {
		return false
	}
	if !strings.Contains(domainPart, ".") ||
		strings.HasPrefix(domainPart, ".") ||
		strings.HasSuffix(domainPart, ".") ||
		strings.Contains(domainPart, " ") {
		return false
	}
	return true
}
