package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The generated middleware must use the real chain contract (return nil to
// continue); nitro.Ctx has no Next method.
func TestMakeMiddleware_GeneratesCompilableBody(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	cmdMakeMiddleware([]string{"Audit"})

	matches, _ := filepath.Glob(filepath.Join(dir, "*", "audit.go"))
	if len(matches) == 0 {
		matches, _ = filepath.Glob(filepath.Join(dir, "*", "*", "audit.go"))
	}
	if len(matches) != 1 {
		t.Fatalf("generated file not found under %s", dir)
	}
	src, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "c.Next()") {
		t.Fatalf("template calls nonexistent c.Next():\n%s", src)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), matches[0], src, 0); err != nil {
		t.Fatalf("generated middleware does not parse: %v", err)
	}
}
