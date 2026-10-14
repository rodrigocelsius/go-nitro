package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasBLinqOptOut(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if hasBLinqOptOut() {
		t.Fatal("expected no opt-out")
	}
	if err := os.WriteFile("AGENTS.md", []byte("This app uses skip BLinq for legacy reasons.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !hasBLinqOptOut() {
		t.Fatal("expected opt-out from AGENTS.md")
	}
}

func TestFindAlternateORMImports(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "repo.go")
	body := "package repo\n\nimport \"gorm.io/gorm\"\n"
	if err := os.WriteFile(src, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	found := findAlternateORMImports(dir)
	ok := false
	for _, f := range found {
		if f == "gorm" {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("expected gorm, got %v", found)
	}
}

func TestDoctorBLinqSQLWithoutModelgen(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("internal/config", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("internal/config/config.go", []byte("package config\n\ntype DBConfig struct{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("main.go", []byte("package main\n\nimport \"gorm.io/gorm\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	alt := findAlternateORMImports(".")
	if len(uniqueSorted(alt)) == 0 {
		t.Fatal("expected alternate ORM")
	}
	if hasBLinqOptOut() {
		t.Fatal("no opt-out yet")
	}
	if dirExists("internal/modelgen") {
		t.Fatal("modelgen should be missing")
	}
}
