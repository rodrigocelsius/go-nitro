package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsNitroRoot(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if !isNitroRoot(root) {
		t.Fatalf("expected Nitro root at %s", root)
	}
	tmp := t.TempDir()
	if isNitroRoot(tmp) {
		t.Fatal("empty dir should not be Nitro root")
	}
}

func TestDetectNitroPathFromCWD(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(root, "cmd", "nitrogen")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	got := detectNitroPath()
	gr, _ := filepath.EvalSymlinks(got)
	rr, _ := filepath.EvalSymlinks(root)
	if gr != rr {
		t.Fatalf("detectNitroPath=%q want %q", got, root)
	}
}

func TestResolveNitroPathExplicitWins(t *testing.T) {
	if got := resolveNitroPath("/explicit"); got != "/explicit" {
		t.Fatalf("got %q", got)
	}
}

func TestModulePathFromGoMod(t *testing.T) {
	if modulePathFromGoMod("module github.com/rodrigocelsius/go-nitro/v2\n\ngo 1.22\n") != "github.com/rodrigocelsius/go-nitro/v2" {
		t.Fatal("parse")
	}
}
