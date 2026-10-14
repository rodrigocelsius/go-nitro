package validator_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

func TestResolveFieldMessage_MsgRuleBeatsMessage(t *testing.T) {
	type User struct {
		CPF string `validate:"required,cpf" msg_cpf:"Informe um CPF válido" message:"CPF obrigatório ou inválido"`
	}

	v := validator.NewValidator()
	v.Register("cpf", func(ctx domain.FieldContext, _ string) error {
		s, _ := ctx.FieldValue.(string)
		if s == "000" || s == "" {
			return &domain.ValidationError{Field: ctx.Path, Message: "invalid cpf", Rule: "cpf"}
		}
		return nil
	})

	errs := v.Validate(User{CPF: "000"})
	if len(errs) != 1 {
		t.Fatalf("want 1 err, got %v", errs)
	}
	if errs[0].Message != "Informe um CPF válido" {
		t.Fatalf("msg_cpf: got %q", errs[0].Message)
	}
	if errs[0].Rule != "cpf" {
		t.Fatalf("rule: got %q", errs[0].Rule)
	}
}

func TestResolveFieldMessage_MessageFallback(t *testing.T) {
	type User struct {
		Name string `validate:"required" message:"Nome obrigatório"`
	}
	errs := validator.ValidateStruct(User{})
	if len(errs) != 1 || errs[0].Message != "Nome obrigatório" {
		t.Fatalf("got %v", errs)
	}
}

func TestResolveFieldMessage_MsgRequired(t *testing.T) {
	type User struct {
		Email string `validate:"required" msg_required:"X" message:"Y"`
	}
	errs := validator.ValidateStruct(User{})
	if len(errs) != 1 {
		t.Fatalf("got %v", errs)
	}
	if errs[0].Message != "X" || errs[0].Rule != "required" {
		t.Fatalf("want msg_required X, got %#v", errs[0])
	}
}

func TestResolveFieldMessage_DefaultUnchanged(t *testing.T) {
	type User struct {
		Name string `validate:"min=3"`
	}
	errs := validator.ValidateStruct(User{Name: "ab"})
	if len(errs) != 1 {
		t.Fatalf("got %v", errs)
	}
	if errs[0].Message != "must be at least 3 characters" {
		t.Fatalf("default broken: %q", errs[0].Message)
	}
}

func TestResolveFieldMessage_Nested(t *testing.T) {
	type Addr struct {
		Street string `json:"street" validate:"required" msg_required:"Rua obrigatória"`
	}
	type User struct {
		Address Addr `json:"address" validate:"required"`
	}
	errs := validator.ValidateStruct(User{Address: Addr{}})
	found := false
	for _, e := range errs {
		if e.Field == "address.street" {
			found = true
			if e.Message != "Rua obrigatória" {
				t.Fatalf("nested msg: %q", e.Message)
			}
		}
	}
	if !found {
		t.Fatalf("missing address.street in %v", errs)
	}
}

func TestExpandMessage_Placeholders(t *testing.T) {
	got := validator.ExpandMessage(
		"{field} falhou em {rule} (param={param}, value={value})",
		"email", "min", "3", "ab",
	)
	want := "email falhou em min (param=3, value=ab)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveFieldMessage_MessageWithPlaceholder(t *testing.T) {
	type User struct {
		Name string `validate:"min=5" message:"{field} precisa de pelo menos {param} chars (got {value})"`
	}
	errs := validator.ValidateStruct(User{Name: "ab"})
	if len(errs) != 1 {
		t.Fatalf("got %v", errs)
	}
	// Field has no `json` tag: the {field} placeholder now uses the same
	// default wire name as JSON/db (snake_case), not the raw Go field name.
	want := "name precisa de pelo menos 5 chars (got ab)"
	if errs[0].Message != want {
		t.Fatalf("got %q want %q", errs[0].Message, want)
	}
}
