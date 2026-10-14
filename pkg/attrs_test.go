package nitro

import "testing"

func TestValidateAttrs_RequiredAndTypes(t *testing.T) {
	defs := []FieldDef{
		{Key: "gramatura", Type: "string", Required: true},
		{Key: "qtd", Type: "number"},
		{Key: "vip", Type: "bool"},
		{Key: "segmento", Type: "select", Options: []string{"a", "b"}},
	}
	if err := ValidateAttrs(defs, map[string]any{}); err == nil {
		t.Fatal("expected required error")
	}
	if err := ValidateAttrs(defs, map[string]any{
		"gramatura": "300g",
		"qtd":       10.0,
		"vip":       true,
		"segmento":  "a",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAttrs(defs, map[string]any{
		"gramatura": "x",
		"segmento":  "z",
	}); err == nil {
		t.Fatal("expected select error")
	}
}
