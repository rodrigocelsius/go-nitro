package dto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate_FieldTag(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "models.go")
	code := `package models

type User struct {
	Name     string ` + "`json:\"name\" nitro:\"public\"`" + `
	Email    string ` + "`json:\"email\" nitro:\"public\"`" + `
	Password string ` + "`json:\"-\"`" + `
	Internal string ` + "`json:\"internal\"`" + `
}
`
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := GenerateFromFile(src, "models")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"type UserPublic struct",
		"Name string",
		"Email string",
		"func (m User) ToPublic()",
		"ToPublicUserSlice",
	} {
		if !contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if contains(out, "Password") || contains(out, "Internal") {
		t.Fatalf("should omit private fields:\n%s", out)
	}
}

func TestGenerate_StructAnnotation(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "models.go")
	code := `package models

//nitro:public
type Product struct {
	Name  string ` + "`json:\"name\"`" + `
	Price float64 ` + "`json:\"price\"`" + `
	Cost  float64 ` + "`json:\"-\" nitro:\"-\"`" + `
}
`
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := GenerateFromFile(src, "models")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "ProductPublic") || !contains(out, "Price") {
		t.Fatalf("%s", out)
	}
	if contains(out, "Cost") {
		t.Fatalf("Cost should be excluded:\n%s", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
