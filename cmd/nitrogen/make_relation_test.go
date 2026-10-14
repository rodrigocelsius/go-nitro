package main

import (
	"os"
	"strings"
	"testing"
)

func TestMakeRelationBelongsTo(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("go.mod", []byte("module example.com/app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"internal/model", "migrations"} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile("internal/model/order.go", []byte(`package model

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

//nitro:table=orders
type Order struct {
	nitro.Model
	Total float64 `+"`json:\"total\" db:\"total\"`"+`
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	cmdMakeRelation([]string{"Order", "belongs_to", "User"})

	b, err := os.ReadFile("internal/model/order.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "UserID") {
		t.Fatalf("missing UserID: %s", b)
	}
	rb, err := os.ReadFile("internal/modelgen/relations_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rb), "OrderBelongsToUser") {
		t.Fatalf("%s", rb)
	}
	entries, _ := os.ReadDir("migrations")
	found := false
	for _, e := range entries {
		if strings.Contains(e.Name(), "user_id") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected migration stub, got %#v", entries)
	}
}

func TestMakeRelationHasMany(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile("go.mod", []byte("module example.com/app\n"), 0644)
	_ = os.MkdirAll("internal/model", 0755)
	_ = os.MkdirAll("migrations", 0755)

	cmdMakeRelation([]string{"User", "has_many", "Order"})

	b, _ := os.ReadFile("internal/model/order.go")
	if !strings.Contains(string(b), "UserID") {
		t.Fatalf("child FK: %s", b)
	}
	rb, _ := os.ReadFile("internal/modelgen/relations_gen.go")
	if !strings.Contains(string(rb), "UserHasManyOrder") {
		t.Fatalf("%s", rb)
	}
}
