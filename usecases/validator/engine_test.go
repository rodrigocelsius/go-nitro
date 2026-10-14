package validator_test

import (
	"errors"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

func TestParseRules(t *testing.T) {
	rules := validator.ParseRules("required,min=3,max=100")
	if len(rules) != 3 {
		t.Fatalf("len=%d", len(rules))
	}
	if rules[0].Name != "required" || rules[1].Name != "min" || rules[1].Param != "3" {
		t.Fatalf("%+v", rules)
	}
}

func TestStructValidator_CustomRuleOnValidate(t *testing.T) {
	v := validator.NewValidator()
	v.Register("cpf", func(ctx domain.FieldContext, param string) error {
		s, _ := ctx.FieldValue.(string)
		if s != "ok" {
			return &domain.ValidationError{Field: ctx.Path, Message: "invalid cpf", Rule: "cpf"}
		}
		return nil
	})

	type User struct {
		CPF string `json:"cpf" validate:"cpf"`
	}
	errs := v.Validate(User{CPF: "bad"})
	if !errs.HasErrors() || errs[0].Rule != "cpf" {
		t.Fatalf("%v", errs)
	}
	if errs := v.Validate(User{CPF: "ok"}); errs.HasErrors() {
		t.Fatalf("%v", errs)
	}
}

func TestStrictUnknown_Default(t *testing.T) {
	type User struct {
		CPF string `json:"cpf" validate:"cpf"`
	}
	errs := validator.ValidateStruct(User{CPF: "x"})
	if len(errs) == 0 || errs[0].Rule != "unknown" {
		t.Fatalf("expected unknown rule error, got %v", errs)
	}
}

func TestStrictUnknown_Disabled(t *testing.T) {
	v := validator.NewValidator()
	v.StrictUnknown = false
	type User struct {
		CPF string `json:"cpf" validate:"cpf"`
	}
	if errs := v.Validate(User{CPF: "x"}); errs.HasErrors() {
		t.Fatalf("strict off should skip unknown, got %v", errs)
	}
}

func TestStructValidator_IsolatedFromDefault(t *testing.T) {
	v := validator.NewValidator()
	v.Register("tmp", func(ctx domain.FieldContext, param string) error {
		return errors.New("tmp")
	})
	if !v.Has("tmp") {
		t.Fatal("expected tmp on instance")
	}
	if validator.DefaultRegistry().Has("tmp") {
		t.Fatal("default must not see instance rule")
	}
}
