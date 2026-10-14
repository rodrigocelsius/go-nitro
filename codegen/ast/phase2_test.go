package ast

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.go")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseFile_MultiNameFields(t *testing.T) {
	// Regression: `FirstName, LastName string` used to silently drop LastName.
	src := `package model

type Order struct {
	FirstName, LastName string
	City                string
}
`
	defs, err := ParseFile(writeTemp(t, src))
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(defs))
	}
	got := map[string]bool{}
	for _, f := range defs[0].Fields {
		got[f.GoName] = true
	}
	for _, want := range []string{"FirstName", "LastName", "City"} {
		if !got[want] {
			t.Errorf("field %q missing; got %v", want, got)
		}
	}
}

func TestParseFile_AliasedTimeImport(t *testing.T) {
	// Regression: `import t "time"` made t.Time classify as KindStruct,
	// breaking downstream generators.
	src := `package model

import t "time"

type Event struct {
	At   t.Time
	Name string
}
`
	defs, err := ParseFile(writeTemp(t, src))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range defs[0].Fields {
		if f.GoName == "At" && f.Kind != KindTime {
			t.Fatalf("t.Time should be KindTime, got kind=%d typeName=%q", f.Kind, f.TypeName)
		}
		if f.GoName == "At" && f.TypeName != "time.Time" {
			t.Fatalf("expected canonical time.Time, got %q", f.TypeName)
		}
	}
}

func TestParseFile_PlainTimeImportStillWorks(t *testing.T) {
	src := `package model

import "time"

type Event struct {
	At time.Time
}
`
	defs, err := ParseFile(writeTemp(t, src))
	if err != nil {
		t.Fatal(err)
	}
	f := defs[0].Fields[0]
	if f.Kind != KindTime || f.TypeName != "time.Time" {
		t.Fatalf("kind=%d typeName=%q", f.Kind, f.TypeName)
	}
}

func TestParseFile_SameFileEmbedExpansion(t *testing.T) {
	src := `package model

import "time"

type Audit struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Product struct {
	Audit
	Name  string
	Price float64
}
`
	defs, err := ParseFile(writeTemp(t, src))
	if err != nil {
		t.Fatal(err)
	}
	var prod StructDef
	for _, d := range defs {
		if d.Name == "Product" {
			prod = d
		}
	}
	got := map[string]bool{}
	for _, f := range prod.Fields {
		got[f.GoName] = true
	}
	for _, want := range []string{"CreatedAt", "UpdatedAt", "Name", "Price"} {
		if !got[want] {
			t.Errorf("promoted field %q missing from Product: %v", want, got)
		}
	}
}

func TestParseFile_EmbedShadowing(t *testing.T) {
	src := `package model

type Base struct {
	ID   int64
	Name string
}

type User struct {
	Base
	Name string // shadows promoted Name
}
`
	defs, err := ParseFile(writeTemp(t, src))
	if err != nil {
		t.Fatal(err)
	}
	var user StructDef
	for _, d := range defs {
		if d.Name == "User" {
			user = d
		}
	}
	count := 0
	for _, f := range user.Fields {
		if f.GoName == "Name" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one Name after shadowing, got %d", count)
	}
}

func TestParseFile_CrossFileEmbedListed(t *testing.T) {
	src := `package model

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

type Item struct {
	nitro.Model
	SKU string
}
`
	defs, err := ParseFile(writeTemp(t, src))
	if err != nil {
		t.Fatal(err)
	}
	item := defs[0]
	found := false
	for _, emb := range item.Embeds {
		if emb == "nitro.Model" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unresolvable embed should be listed in Embeds: %+v", item.Embeds)
	}
	names := map[string]bool{}
	for _, f := range item.Fields {
		names[f.GoName] = true
	}
	if len(names) != 1 || !names["SKU"] {
		t.Fatalf("unexpected fields: %v", names)
	}
}

func TestParseFile_RecursiveEmbedCycleSafe(t *testing.T) {
	src := `package model

type A struct {
	B
	AField string
}

type B struct {
	A
	BField string
}
`
	if _, err := ParseFile(writeTemp(t, src)); err != nil {
		t.Fatalf("cycle must not hang or panic: %v", err)
	}
}
