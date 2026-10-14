package validator_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

func TestApplyDefaults_BoolOmittedVsFalse(t *testing.T) {
	type in struct {
		Active bool   `json:"active" default:"true"`
		Name   string `json:"name"`
	}

	var a in
	if err := validator.UnmarshalJSONDefaults([]byte(`{"name":"x"}`), &a); err != nil {
		t.Fatal(err)
	}
	if !a.Active || a.Name != "x" {
		t.Fatalf("%+v", a)
	}

	var b in
	if err := validator.UnmarshalJSONDefaults([]byte(`{"name":"x","active":false}`), &b); err != nil {
		t.Fatal(err)
	}
	if b.Active {
		t.Fatal("explicit false must win")
	}
}

func TestApplyDefaults_ValidateTag(t *testing.T) {
	type in struct {
		N int `json:"n" validate:"default=3"`
	}
	var v in
	if err := validator.UnmarshalJSONDefaults([]byte(`{}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.N != 3 {
		t.Fatalf("%d", v.N)
	}
}

func TestApplyDefaults_Slice(t *testing.T) {
	type row struct {
		Active bool `json:"active" default:"true"`
	}
	var rows []row
	if err := validator.UnmarshalJSONDefaults([]byte(`[{},{"active":false}]`), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[0].Active || rows[1].Active {
		t.Fatalf("%+v", rows)
	}
}
