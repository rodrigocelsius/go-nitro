package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileRemovesGitkeep(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "middleware")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dir, ".gitkeep")
	if err := os.WriteFile(keep, nil, 0644); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "auth.go")
	if err := writeFile(path, "package middleware\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); !os.IsNotExist(err) {
		t.Fatalf("expected .gitkeep removed, got err=%v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestWriteFileKeepsGitkeepWhenWritingGitkeep(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "model")
	keep := filepath.Join(dir, ".gitkeep")
	if err := writeFile(keep, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("expected .gitkeep to remain: %v", err)
	}
}

func TestProjectTemplatesOmitOptionalDirs(t *testing.T) {
	files := projectTemplates("app", "example.com/app", "default")
	for _, p := range []string{
		"internal/seed/.gitkeep",
		"internal/factory/.gitkeep",
		"internal/middleware/.gitkeep",
		"internal/job/.gitkeep",
		".env.dev",
		".env.stage",
		".env.prod",
	} {
		if _, ok := files[p]; ok {
			t.Errorf("unexpected path in new scaffold: %s", p)
		}
	}
	for _, p := range []string{
		"internal/model/.gitkeep",
		"internal/usecase/.gitkeep",
		"internal/repository/.gitkeep",
		"migrations/.gitkeep",
		".env",
		".env.example",
	} {
		if _, ok := files[p]; !ok {
			t.Errorf("missing core path in new scaffold: %s", p)
		}
	}
}

func TestParseMakeNameArgs_TenantFlagAddsTenantID(t *testing.T) {
	_, _, fields, _, _ := parseMakeNameArgs([]string{"Invoice", "total:float64", "--tenant"})
	var found bool
	for _, f := range fields {
		if f.Name == "TenantID" || fieldJSONName(f) == "tenant_id" {
			found = true
		}
	}
	if !found {
		t.Fatalf("--tenant must add a tenant_id field, got %+v", fields)
	}
}
