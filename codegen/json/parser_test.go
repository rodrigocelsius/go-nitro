package gen

import "testing"

func TestParseFile_Order(t *testing.T) {
	defs, err := ParseFile("testdata/order.go")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	byName := make(map[string]StructDef)
	for _, d := range defs {
		byName[d.Name] = d
	}

	order, ok := byName["Order"]
	if !ok {
		t.Fatal("Order struct not found")
	}

	fieldsByName := make(map[string]Field)
	for _, f := range order.Fields {
		fieldsByName[f.GoName] = f
	}

	// Secret has an explicit `json:"-"`, so it must not appear.
	if _, exists := fieldsByName["Secret"]; exists {
		t.Fatal("Secret field should be excluded (json:\"-\")")
	}

	cases := []struct {
		goName       string
		wantJSONName string
		wantKind     FieldKind
		wantElemKind FieldKind
	}{
		{"ID", "id", KindInt, 0},
		{"Total", "total", KindFloat, 0},
		{"Active", "active", KindBool, 0},
		{"Address", "address", KindStruct, 0},
		{"Billing", "billing", KindStructPtr, 0},
		{"Items", "items", KindSlice, KindStruct},
		{"Tags", "tags", KindSlice, KindString},
		// Internal has no `json` tag: defaults to snake_case(GoName).
		{"Internal", "internal", KindString, 0},
	}

	for _, tc := range cases {
		f, ok := fieldsByName[tc.goName]
		if !ok {
			t.Fatalf("field %s not found", tc.goName)
		}
		if f.JSONName != tc.wantJSONName {
			t.Fatalf("field %s: expected JSON name %q, got %q", tc.goName, tc.wantJSONName, f.JSONName)
		}
		if f.Kind != tc.wantKind {
			t.Fatalf("field %s: expected kind %v, got %v", tc.goName, tc.wantKind, f.Kind)
		}
		if tc.wantElemKind != 0 && f.ElemKind != tc.wantElemKind {
			t.Fatalf("field %s: expected elem kind %v, got %v", tc.goName, tc.wantElemKind, f.ElemKind)
		}
	}
}

func TestParseFile_AddressAndItem(t *testing.T) {
	defs, err := ParseFile("testdata/order.go")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	byName := make(map[string]StructDef)
	for _, d := range defs {
		byName[d.Name] = d
	}

	addr, ok := byName["Address"]
	if !ok || len(addr.Fields) != 2 {
		t.Fatalf("expected Address with 2 fields, got %+v", addr)
	}

	item, ok := byName["Item"]
	if !ok || len(item.Fields) != 2 {
		t.Fatalf("expected Item with 2 fields, got %+v", item)
	}
}
