package validator_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

func TestStopOnFirstError(t *testing.T) {
	type User struct {
		A string `validate:"required"`
		B string `validate:"required"`
	}
	v := validator.NewValidator()
	v.SetOptions(validator.Options{StopOnFirstError: true, StrictUnknown: true})

	errs := v.Validate(User{})
	if len(errs) != 1 {
		t.Fatalf("want 1 error, got %d: %v", len(errs), errs)
	}
	// Field has no `json` tag: the error path now uses the default wire
	// name (snake_case), same convention as JSON/db.
	if errs[0].Field != "a" {
		t.Fatalf("want first field \"a\", got %q", errs[0].Field)
	}
}

func TestTypeMetaCache_Consistent(t *testing.T) {
	type Row struct {
		Name  string `json:"name" validate:"required,min=2"`
		Email string `json:"email" validate:"required,email"`
	}
	validator.ClearTypeMetaCache()
	a := validator.ValidateStruct(Row{Name: "x", Email: "bad"})
	b := validator.ValidateStruct(Row{Name: "x", Email: "bad"})
	if len(a) != len(b) || len(a) == 0 {
		t.Fatalf("inconsistent: %v vs %v", a, b)
	}
	for i := range a {
		if a[i].Field != b[i].Field || a[i].Rule != b[i].Rule {
			t.Fatalf("mismatch %v vs %v", a[i], b[i])
		}
	}
}
