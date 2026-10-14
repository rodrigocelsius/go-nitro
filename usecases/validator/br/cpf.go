package br

import "github.com/rodrigocelsius/go-nitro/domain/v2"

// CPF validates Brazilian CPF.
func CPF(ctx domain.FieldContext, _ string) error {
	s, _ := ctx.FieldValue.(string)
	if s == "" {
		return nil
	}
	if !validCPF(s) {
		return &domain.ValidationError{Field: ctx.Path, Message: "invalid cpf", Rule: "cpf"}
	}
	return nil
}

func validCPF(s string) bool {
	digits := digitsOnly(s)
	if len(digits) != 11 || allSame(digits) {
		return false
	}
	d := make([]int, 11)
	for i := 0; i < 11; i++ {
		d[i] = int(digits[i] - '0')
	}
	sum := 0
	for i := 0; i < 9; i++ {
		sum += d[i] * (10 - i)
	}
	r := (sum * 10) % 11
	if r == 10 {
		r = 0
	}
	if r != d[9] {
		return false
	}
	sum = 0
	for i := 0; i < 10; i++ {
		sum += d[i] * (11 - i)
	}
	r = (sum * 10) % 11
	if r == 10 {
		r = 0
	}
	return r == d[10]
}

func allSame(s string) bool {
	if s == "" {
		return true
	}
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}
