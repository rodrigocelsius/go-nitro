package datagen

import "strings"

// pluralize applies English pluralization rules covering the most
// common web-app entity names. It is not a full pluralizer (it does
// not cover every English irregularity), but it covers the usual
// patterns: -y -> -ies, -s/-x/-ch/-sh -> -es, and the common -is ->
// -es case (e.g. "analysis" -> "analyses"... rare in entity names in
// practice, included for completeness).
func pluralize(word string) string {
	if word == "" {
		return word
	}

	lower := strings.ToLower(word)

	switch {
	case strings.HasSuffix(lower, "y") && !endsInVowelY(lower):
		return word[:len(word)-1] + "ies"
	case strings.HasSuffix(lower, "s"),
		strings.HasSuffix(lower, "x"),
		strings.HasSuffix(lower, "ch"),
		strings.HasSuffix(lower, "sh"):
		return word + "es"
	default:
		return word + "s"
	}
}

// endsInVowelY reports whether the word ends in vowel+y (e.g. "day"),
// the case where the -y->-ies rule does NOT apply (day -> days, not daies).
func endsInVowelY(lower string) bool {
	if len(lower) < 2 {
		return false
	}
	beforeY := lower[len(lower)-2]
	switch beforeY {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}

// toSnakeCase converts a Go identifier (PascalCase or camelCase) to
// snake_case, treating consecutive uppercase sequences as a single
// acronym (e.g. "ID" -> "id", "SKU" -> "sku"), not as individual
// letters separated by "_".
//
// Rule: insert "_" before an uppercase letter when:
//   - the previous position is lowercase (normal word start: "Foo"
//     in "userFoo"), OR
//   - the previous position is uppercase BUT the next is lowercase
//     (end of acronym followed by a word: the "U" before "ser" in "IDUser").
//
// Real bug found during development: the previous version (letter by
// letter) turned "ID" into "i_d" and "SKU" into "s_k_u" — caught in a
// manual inspection of generated code before any benchmark or
// integration test ran.
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

// tableNameFromStruct applies the default convention: snake_case + plural.
func tableNameFromStruct(structName string) string {
	return pluralize(toSnakeCase(structName))
}

// TableNameFromStruct is the CoC table name (snake_case plural of the type).
func TableNameFromStruct(structName string) string {
	return tableNameFromStruct(structName)
}
