package gen

import "strings"

// toSnakeCase converts a Go identifier (PascalCase or camelCase) to
// snake_case, treating consecutive uppercase sequences as a single acronym
// (e.g. "ID" -> "id", "SKU" -> "sku"), not as individual letters separated
// by "_". Used as the default JSON key for fields without an explicit
// `json` tag — same convention as codegen/data (BLinq metamodel),
// blinq/scan (SELECT scan) and usecases/transform (runtime rename, see
// usecases/jsontag). Kept as a local copy (this module intentionally has no
// dependency beyond codegen/ast); keep in sync via the shared golden test
// vectors in naming_test.go.
func toSnakeCase(s string) string {
	if s == "" {
		return s
	}

	var b strings.Builder
	runes := []rune(s)

	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'
		if isUpper {
			prevLower := i > 0 && isLowerRune(runes[i-1])
			nextLower := i+1 < len(runes) && isLowerRune(runes[i+1])
			startsNewWord := i > 0 && (prevLower || (nextLower && !prevLower))
			if startsNewWord {
				b.WriteByte('_')
			}
			b.WriteByte(byte(r - 'A' + 'a'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isLowerRune(r rune) bool {
	return r >= 'a' && r <= 'z'
}
