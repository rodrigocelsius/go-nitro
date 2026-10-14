package ast

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile_SimpleStruct(t *testing.T) {
	content := `
package test

type User struct {
	Name string ` + "`json:\"name\"`" + `
	Age  int    ` + "`json:\"age\"`" + `
}
`
	tmp := writeTempFile(t, "test.go", content)
	defer os.Remove(tmp)

	defs, err := ParseFile(tmp)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(defs) != 1 {
		t.Fatalf("got %d structs, want 1", len(defs))
	}

	def := defs[0]
	if def.Name != "User" {
		t.Errorf("Name = %q, want User", def.Name)
	}
	if len(def.Fields) != 2 {
		t.Fatalf("got %d fields, want 2", len(def.Fields))
	}

	if def.Fields[0].GoName != "Name" || def.Fields[0].Kind != KindString {
		t.Errorf("Field 0 = %v, want Name/string", def.Fields[0])
	}
	if def.Fields[1].GoName != "Age" || def.Fields[1].Kind != KindInt {
		t.Errorf("Field 1 = %v, want Age/int", def.Fields[1])
	}
}

func TestParseFile_MultipleTags(t *testing.T) {
	content := `
package test

type Product struct {
	Name  string ` + "`json:\"name\" db:\"product_name\"`" + `
	Price float64 ` + "`json:\"price\" db:\"price_cents\"`" + `
}
`
	tmp := writeTempFile(t, "test.go", content)
	defer os.Remove(tmp)

	defs, err := ParseFile(tmp)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(defs) != 1 {
		t.Fatalf("got %d structs, want 1", len(defs))
	}

	nameField := defs[0].Fields[0]
	if nameField.Tags["json"] != "name" {
		t.Errorf("json tag = %q, want name", nameField.Tags["json"])
	}
	if nameField.Tags["db"] != "product_name" {
		t.Errorf("db tag = %q, want product_name", nameField.Tags["db"])
	}
}

func TestParseFile_NoTags(t *testing.T) {
	content := `
package test

type Config struct {
	Host string
	Port int
}
`
	tmp := writeTempFile(t, "test.go", content)
	defer os.Remove(tmp)

	defs, err := ParseFile(tmp)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(defs) != 1 {
		t.Fatalf("got %d structs, want 1", len(defs))
	}

	for _, f := range defs[0].Fields {
		if f.Tags != nil {
			t.Errorf("Field %s should have nil tags", f.GoName)
		}
	}
}

func TestParseFile_PointerField(t *testing.T) {
	content := `
package test

type Inner struct {
	Value string
}

type Outer struct {
	Ptr *Inner ` + "`json:\"ptr\"`" + `
}
`
	tmp := writeTempFile(t, "test.go", content)
	defer os.Remove(tmp)

	defs, err := ParseFile(tmp)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	outer := findDef(defs, "Outer")
	if outer == nil {
		t.Fatal("Outer struct not found")
	}

	if len(outer.Fields) != 1 {
		t.Fatalf("got %d fields, want 1", len(outer.Fields))
	}

	f := outer.Fields[0]
	if f.Kind != KindStructPtr {
		t.Errorf("Kind = %v, want KindStructPtr", f.Kind)
	}
	if f.TypeName != "*Inner" {
		t.Errorf("TypeName = %q, want *Inner", f.TypeName)
	}
}

func TestParseFile_SliceField(t *testing.T) {
	content := `
package test

type User struct {
	Tags []string ` + "`json:\"tags\"`" + `
}
`
	tmp := writeTempFile(t, "test.go", content)
	defer os.Remove(tmp)

	defs, err := ParseFile(tmp)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	user := findDef(defs, "User")
	if user == nil {
		t.Fatal("User struct not found")
	}

	f := user.Fields[0]
	if f.Kind != KindSlice {
		t.Errorf("Kind = %v, want KindSlice", f.Kind)
	}
	if f.ElemKind != KindString {
		t.Errorf("ElemKind = %v, want KindString", f.ElemKind)
	}
}

func TestParseFile_AllTypes(t *testing.T) {
	content := `
package test

type AllTypes struct {
	Str   string
	Int   int
	Int64 int64
	Uint  uint
	Float float64
	Bool  bool
}
`
	tmp := writeTempFile(t, "test.go", content)
	defer os.Remove(tmp)

	defs, err := ParseFile(tmp)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	all := findDef(defs, "AllTypes")
	if all == nil {
		t.Fatal("AllTypes struct not found")
	}

	if len(all.Fields) != 6 {
		t.Fatalf("got %d fields, want 6", len(all.Fields))
	}

	expected := map[string]FieldKind{
		"Str":   KindString,
		"Int":   KindInt,
		"Int64": KindInt,
		"Uint":  KindInt,
		"Float": KindFloat,
		"Bool":  KindBool,
	}

	for _, f := range all.Fields {
		if f.Kind != expected[f.GoName] {
			t.Errorf("Field %s Kind = %v, want %v", f.GoName, f.Kind, expected[f.GoName])
		}
	}
}

func TestParsePackageName(t *testing.T) {
	content := `package mypackage

type Foo struct{}
`
	tmp := writeTempFile(t, "test.go", content)
	defer os.Remove(tmp)

	name, err := ParsePackageName(tmp)
	if err != nil {
		t.Fatalf("ParsePackageName error: %v", err)
	}

	if name != "mypackage" {
		t.Errorf("PackageName = %q, want mypackage", name)
	}
}

func TestParsePackageName_Error(t *testing.T) {
	_, err := ParsePackageName("/nonexistent/file.go")
	if err == nil {
		t.Error("ParsePackageName should error on nonexistent file")
	}
}

func TestTagValue(t *testing.T) {
	tests := []struct {
		name    string
		field   Field
		key     string
		wantVal string
		wantOk  bool
	}{
		{
			name:    "simple tag",
			field:   Field{Tags: map[string]string{"json": "name"}},
			key:     "json",
			wantVal: "name",
			wantOk:  true,
		},
		{
			name:    "tag with omitempty",
			field:   Field{Tags: map[string]string{"json": "name,omitempty"}},
			key:     "json",
			wantVal: "name",
			wantOk:  true,
		},
		{
			name:    "tag not found",
			field:   Field{Tags: map[string]string{"db": "col"}},
			key:     "json",
			wantVal: "",
			wantOk:  false,
		},
		{
			name:    "nil tags",
			field:   Field{Tags: nil},
			key:     "json",
			wantVal: "",
			wantOk:  false,
		},
		{
			name:    "dash tag",
			field:   Field{Tags: map[string]string{"json": "-"}},
			key:     "json",
			wantVal: "",
			wantOk:  false,
		},
		{
			name:    "empty value",
			field:   Field{Tags: map[string]string{"json": ""}},
			key:     "json",
			wantVal: "",
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotOk := TagValue(tt.field, tt.key)
			if gotVal != tt.wantVal || gotOk != tt.wantOk {
				t.Errorf("TagValue() = (%q, %v), want (%q, %v)", gotVal, gotOk, tt.wantVal, tt.wantOk)
			}
		})
	}
}

func TestExtractAllTagValues(t *testing.T) {
	tests := []struct {
		tag  string
		want map[string]string
	}{
		{
			tag:  `json:"name" db:"col"`,
			want: map[string]string{"json": "name", "db": "col"},
		},
		{
			tag:  `json:"name,omitempty"`,
			want: map[string]string{"json": "name,omitempty"},
		},
		{
			tag:  ``,
			want: nil,
		},
	}

	for _, tt := range tests {
		got := extractAllTagValues(tt.tag)
		if len(got) != len(tt.want) {
			t.Errorf("extractAllTagValues(%q) = %v (len=%d), want %v (len=%d)", tt.tag, got, len(got), tt.want, len(tt.want))
			continue
		}
		for k, v := range tt.want {
			if got[k] != v {
				t.Errorf("extractAllTagValues(%q)[%q] = %q, want %q", tt.tag, k, got[k], v)
			}
		}
	}
}

func TestTrimBackticks(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"`hello`", "hello"},
		{"`", "`"},
		{"hello", "hello"},
		{"``", ""},
	}

	for _, tt := range tests {
		got := trimBackticks(tt.input)
		if got != tt.want {
			t.Errorf("trimBackticks(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveType(t *testing.T) {
	content := `
package test

type Inner struct{}

type Types struct {
	S    string
	I    int
	F    float64
	B    bool
	P    *Inner
	PS   *string
	Sli  []string
}
`
	tmp := writeTempFile(t, "test.go", content)
	defer os.Remove(tmp)

	defs, err := ParseFile(tmp)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	types := findDef(defs, "Types")
	if types == nil {
		t.Fatal("Types struct not found")
	}

	expected := map[string]FieldKind{
		"S":   KindString,
		"I":   KindInt,
		"F":   KindFloat,
		"B":   KindBool,
		"P":   KindStructPtr,
		"PS":  KindString,
		"Sli": KindSlice,
	}

	for _, f := range types.Fields {
		if f.Kind != expected[f.GoName] {
			t.Errorf("Field %s Kind = %v, want %v", f.GoName, f.Kind, expected[f.GoName])
		}
	}
}

func TestParseFile_SkipNonStruct(t *testing.T) {
	content := `
package test

type MyInt int

type Valid struct {
	Name string
}
`
	tmp := writeTempFile(t, "test.go", content)
	defer os.Remove(tmp)

	defs, err := ParseFile(tmp)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(defs) != 1 {
		t.Errorf("got %d structs, want 1 (should skip non-struct types)", len(defs))
	}
	if defs[0].Name != "Valid" {
		t.Errorf("Name = %q, want Valid", defs[0].Name)
	}
}

func TestParseFile_InvalidFile(t *testing.T) {
	_, err := ParseFile("/nonexistent/file.go")
	if err == nil {
		t.Error("ParseFile should error on nonexistent file")
	}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	return path
}

func findDef(defs []StructDef, name string) *StructDef {
	for i := range defs {
		if defs[i].Name == name {
			return &defs[i]
		}
	}
	return nil
}
