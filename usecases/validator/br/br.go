// Package br provides Brazilian document validators (CPF, CNPJ, phone, CEP, UF).
//
// Opt-in plugin of usecases/validator — never registered by NewValidator/registerBuiltins.
// Prefer the facade: nitro.RegisterBRValidators(v). App-specific rules (SKU, …) stay in the app.
//
//	br.Register(app.Validator())
package br

import (
	"regexp"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

var (
	rePhoneBR = regexp.MustCompile(`^\d{10,11}$`)
	reCEP     = regexp.MustCompile(`^\d{5}-?\d{3}$`)
	reUF      = regexp.MustCompile(`(?i)^(AC|AL|AP|AM|BA|CE|DF|ES|GO|MA|MT|MS|MG|PA|PB|PR|PE|PI|RJ|RN|RS|RO|RR|SC|SP|SE|TO)$`)
)

// Registrable is a validator that accepts Register.
type Registrable interface {
	Register(name string, fn domain.ValidatorFunc)
}

// Register adds cpf, cnpj, phone_br, cep, uf to the validator.
func Register(v Registrable) {
	v.Register("cpf", CPF)
	v.Register("cnpj", CNPJ)
	v.Register("phone_br", PhoneBR)
	v.Register("cep", CEP)
	v.Register("uf", UF)
}

// PhoneBR validates 10–11 digit Brazilian phone (digits only).
func PhoneBR(ctx domain.FieldContext, _ string) error {
	s, _ := ctx.FieldValue.(string)
	if s == "" {
		return nil
	}
	d := digitsOnly(s)
	if !rePhoneBR.MatchString(d) {
		return &domain.ValidationError{Field: ctx.Path, Message: "invalid brazilian phone", Rule: "phone_br"}
	}
	return nil
}

// CEP validates #####-### or ########.
func CEP(ctx domain.FieldContext, _ string) error {
	s, _ := ctx.FieldValue.(string)
	if s == "" {
		return nil
	}
	s = strings.TrimSpace(s)
	if !reCEP.MatchString(s) {
		return &domain.ValidationError{Field: ctx.Path, Message: "invalid cep", Rule: "cep"}
	}
	return nil
}

// UF validates Brazilian state code.
func UF(ctx domain.FieldContext, _ string) error {
	s, _ := ctx.FieldValue.(string)
	if s == "" {
		return nil
	}
	if !reUF.MatchString(strings.TrimSpace(s)) {
		return &domain.ValidationError{Field: ctx.Path, Message: "invalid uf", Rule: "uf"}
	}
	return nil
}

func digitsOnly(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			b = append(b, c)
		}
	}
	return string(b)
}
