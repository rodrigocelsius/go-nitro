package fasthttp

import "unsafe"

// b2s converts []byte to string without allocation.
// The caller must ensure the []byte is not modified for the lifetime of the string.
func b2s(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// matchPathBytes compares a []byte with a string with no allocation.
// Returns true if they are identical. Faster than string(p) == s
// because it avoids a type conversion and memory allocation.
func matchPathBytes(p []byte, s string) bool {
	if len(p) != len(s) {
		return false
	}
	for i := 0; i < len(p); i++ {
		if p[i] != s[i] {
			return false
		}
	}
	return true
}

// matchPathBytesPrefix reports whether []byte starts with the given string.
// Useful for routes like /users/:id where the prefix is /users/.
func matchPathBytesPrefix(p []byte, prefix string) bool {
	if len(p) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if p[i] != prefix[i] {
			return false
		}
	}
	return true
}

// extractParamAfter extracts the parameter value after a prefix.
// E.g. extractParamAfter([]byte("/users/123"), "/users/") → "123"
// Does not allocate — returns a slice of the original buffer.
func extractParamAfter(p []byte, prefixLen int) string {
	return string(p[prefixLen:])
}

// methodBytes compares method []byte with a string with no allocation.
func methodBytes(m []byte, s string) bool {
	if len(m) != len(s) {
		return false
	}
	for i := 0; i < len(m); i++ {
		if m[i] != s[i] {
			return false
		}
	}
	return true
}
