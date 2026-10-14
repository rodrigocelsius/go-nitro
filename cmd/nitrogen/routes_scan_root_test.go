package main

import (
	"os"
	"path/filepath"
	"testing"
)

// `nitrogen routes` / `nitrogen openapi` default to root "." — whose name
// starts with a dot — and must still scan the project (hidden subdirectories
// stay skipped).
func TestCollectGoFilesDefaultDotRoot(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	for path, src := range map[string]string{
		"internal/handler/routes.go": "package handler\n\nfunc Register(app *nitro.App) {\n\tapp.Get(\"/health\", Health)\n}\n",
		".cache/skip.go":             "package skip\n\nfunc x(app *nitro.App) { app.Get(\"/hidden\", nil) }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := collectGoFiles(".")
	if len(files) != 1 || filepath.ToSlash(files[0]) != "internal/handler/routes.go" {
		t.Fatalf("collectGoFiles(\".\") = %v", files)
	}
	routes := scanProjectRoutes(".")
	if len(routes) != 1 || routes[0].path != "/health" {
		t.Fatalf("scanProjectRoutes(\".\") = %+v", routes)
	}
}
