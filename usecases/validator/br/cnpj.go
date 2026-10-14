package br

import (
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// CNPJ validates Brazilian CNPJ (numeric and alphanumeric / RFB 2026).
//
// Format: 12 base chars [0-9A-Z] + 2 numeric check digits.
// Mask (. / -) is ignored; letters are uppercased.
// Check digits use mod 11 with char value = ASCII - 48 (so digits stay 0–9;
// A=17 … Z=42), same weights as the classic algorithm — numeric CNPJs keep
// working unchanged.
func CNPJ(ctx domain.FieldContext, _ string) error {
	s, _ := ctx.FieldValue.(string)
	if s == "" {
		return nil
	}
	if !validCNPJ(s) {
		return &domain.ValidationError{Field: ctx.Path, Message: "invalid cnpj", Rule: "cnpj"}
	}
	return nil
}

func validCNPJ(s string) bool {
	chars := normalizeCNPJ(s)
	if len(chars) != 14 || allSame(chars) {
		return false
	}
	for i := 0; i < 12; i++ {
		c := chars[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	if chars[12] < '0' || chars[12] > '9' || chars[13] < '0' || chars[13] > '9' {
		return false
	}

	v := make([]int, 14)
	for i := 0; i < 14; i++ {
		v[i] = int(chars[i]) - 48 // ASCII - '0'
	}

	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum := 0
	for i := 0; i < 12; i++ {
		sum += v[i] * w1[i]
	}
	dv1 := cnpjDV(sum)
	if dv1 != v[12] {
		return false
	}

	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum = 0
	for i := 0; i < 13; i++ {
		sum += v[i] * w2[i]
	}
	return cnpjDV(sum) == v[13]
}

func cnpjDV(sum int) int {
	r := sum % 11
	if r < 2 {
		return 0
	}
	return 11 - r
}

// normalizeCNPJ strips mask and uppercases letters.
func normalizeCNPJ(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	b := make([]byte, 0, 14)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9', c >= 'A' && c <= 'Z':
			b = append(b, c)
		case c == '.' || c == '/' || c == '-' || c == ' ':
			// mask
		default:
			return "" // invalid char → fail length
		}
	}
	return string(b)
}
