package validator_test

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

func TestUnmarshalJSONDefaults_Transforms(t *testing.T) {
	type Req struct {
		Name  string `json:"name" transform:"trim,titlecase=pt"`
		Phone string `json:"phone" transform:"digits"`
		State string `json:"state" default:"sp" transform:"upper"`
	}
	var v Req
	err := validator.UnmarshalJSONDefaults([]byte(`{"name":"  joão da silva ","phone":"(11) 98877-6655"}`), &v)
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "João da Silva" {
		t.Fatalf("name=%q", v.Name)
	}
	if v.Phone != "11988776655" {
		t.Fatalf("phone=%q", v.Phone)
	}
	if v.State != "SP" {
		t.Fatalf("state=%q", v.State)
	}
}
