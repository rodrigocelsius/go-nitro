package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestE2E_NewBasic scaffolds a basic app in a tempdir (CLI smoke).
func TestE2E_NewBasic(t *testing.T) {
	root := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	nitroRoot := filepath.Clean(filepath.Join(cwd, "../.."))
	if _, err := os.Stat(filepath.Join(nitroRoot, "pkg")); err != nil {
		// running from cmd/nitrogen → parent is repo root
		nitroRoot = filepath.Clean(filepath.Join(cwd, ".."))
	}
	if _, err := os.Stat(filepath.Join(nitroRoot, "pkg")); err != nil {
		t.Skip("cannot locate nitro pkg for --nitro-path")
	}

	cmdNew([]string{"demoapp", "--basic", "--module", "example.com/demoapp", "--nitro-path", nitroRoot})

	for _, rel := range []string{
		"demoapp/go.mod",
		"demoapp/main.go",
		"demoapp/AGENTS.md",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}

// TestE2E_AiInitRefresh forces ai:init into an existing tree.
func TestE2E_AiInitRefresh(t *testing.T) {
	dir := t.TempDir()
	written, skipped, err := applyAiInit(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Fatalf("skipped=%v", skipped)
	}
	if len(written) == 0 {
		t.Fatal("expected written files")
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
}
