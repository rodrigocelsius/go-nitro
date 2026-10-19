package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceTarget_IsRelative(t *testing.T) {
	base := t.TempDir()
	nitro := filepath.Join(base, "go-nitro")
	app := filepath.Join(base, "work", "svc", "api")
	if got := replaceTarget(app, nitro); got != "../../../go-nitro" {
		t.Fatalf("replaceTarget = %q", got)
	}
	inside := filepath.Join(nitro, "examples", "app")
	if got := replaceTarget(nitro, inside); got != "./examples/app" {
		t.Fatalf("replaceTarget child = %q", got)
	}
}

func TestAppendNitroReplace_WritesPortablePaths(t *testing.T) {
	base := t.TempDir()
	nitro := filepath.Join(base, "go-nitro")
	app := filepath.Join(base, "apps", "demo")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	mod := filepath.Join(app, "go.mod")
	if err := os.WriteFile(mod, []byte("module example.com/demo\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendNitroReplace(mod, nitro); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(mod)
	if strings.Contains(string(b), base) {
		t.Fatalf("go.mod must not contain absolute paths:\n%s", b)
	}
	if !strings.Contains(string(b), "=> ../../go-nitro/pkg") {
		t.Fatalf("expected relative replace:\n%s", b)
	}
}
