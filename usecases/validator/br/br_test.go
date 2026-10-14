package br_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator/br"
)

func TestRegister_PhoneCEP_UF_CPF(t *testing.T) {
	v := usecases.NewValidator()
	br.Register(v)

	type Row struct {
		Phone string `json:"phone" validate:"phone_br"`
		CEP   string `json:"cep" validate:"cep"`
		UF    string `json:"uf" validate:"uf"`
		CPF   string `json:"cpf" validate:"cpf"`
	}

	ok := Row{Phone: "11988776655", CEP: "01310-100", UF: "SP", CPF: "529.982.247-25"}
	if errs := v.Validate(ok); errs.HasErrors() {
		t.Fatalf("expected ok: %v", errs)
	}

	bad := Row{Phone: "123", CEP: "x", UF: "XX", CPF: "000"}
	if errs := v.Validate(bad); !errs.HasErrors() {
		t.Fatal("expected errors")
	}
	_ = domain.ValidationErrors{}
}

func TestCNPJ_NumericAndAlphanumeric(t *testing.T) {
	v := usecases.NewValidator()
	br.Register(v)

	type Row struct {
		CNPJ string `json:"cnpj" validate:"cnpj"`
	}

	cases := []struct {
		in   string
		want bool
	}{
		{"04.252.011/0001-10", true},  // classic numeric
		{"11222333000181", true},      // unmasked numeric
		{"12.ABC.345/01DE-35", true},  // RFB alphanumeric example
		{"12abc34501de35", true},      // lowercase accepted
		{"12.ABC.345/01DE-00", false}, // bad DV
		{"00000000000000", false},     // all same
		{"12.ABC.345/01DE-3", false},  // short
		{"12.ABC.345/01DE-3X", false}, // non-digit DV
		{"", true},                    // empty skipped
	}
	for _, tc := range cases {
		errs := v.Validate(Row{CNPJ: tc.in})
		ok := !errs.HasErrors()
		if ok != tc.want {
			t.Errorf("cnpj %q: ok=%v want %v errs=%v", tc.in, ok, tc.want, errs)
		}
	}
}
