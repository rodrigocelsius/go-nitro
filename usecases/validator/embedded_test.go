package validator

import (
	"testing"
)

type EmbeddedBase struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"omitempty,email"`
	Level int    `json:"level" default:"3"`
}

type embeddedPrivate struct {
	Code string `json:"code" validate:"required,min=3"`
}

type embeddedInput struct {
	EmbeddedBase
	embeddedPrivate
	Secret string `json:"secret" validate:"required"`
}

type namedEmbed struct {
	EmbeddedBase `json:"base"`
}

// encoding/json promotes the fields of an embedded struct without a JSON
// name, so their errors must use the parent's path ("name", not
// "embedded_base.name"), including for unexported embedded structs.
func TestEmbeddedStructFieldsUseThePromotedPath(t *testing.T) {
	var v embeddedInput
	if err := UnmarshalJSONDefaults([]byte(`{"secret":"x","email":"nope","code":"ab"}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.Level != 3 {
		t.Fatalf("default on embedded field: level = %d", v.Level)
	}
	got := map[string]string{}
	for _, e := range ValidateStruct(&v) {
		got[e.Field] = e.Rule
	}
	want := map[string]string{"name": "required", "email": "email", "code": "min"}
	if len(got) != len(want) {
		t.Fatalf("errors = %v, want %v", got, want)
	}
	for f, r := range want {
		if got[f] != r {
			t.Fatalf("errors = %v, want %v", got, want)
		}
	}
}

func TestNamedEmbeddedStructKeepsItsPrefix(t *testing.T) {
	errs := ValidateStruct(&namedEmbed{})
	if len(errs) != 1 || errs[0].Field != "base.name" {
		t.Fatalf("errors = %v, want base.name", errs)
	}
}
