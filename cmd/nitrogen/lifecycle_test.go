package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseUpArgs(t *testing.T) {
	o := parseUpArgs([]string{"--seed", "--migrate-only"})
	if !o.seed || !o.migrateOnly || !o.noDev {
		t.Fatalf("%+v", o)
	}
	o2 := parseUpArgs([]string{"--no-dev"})
	if !o2.noDev || o2.seed {
		t.Fatalf("%+v", o2)
	}
}

func TestParseGoVersion(t *testing.T) {
	maj, min, ok := parseGoVersion("go version go1.25.3 linux/amd64")
	if !ok || maj != 1 || min != 25 {
		t.Fatalf("%d %d %v", maj, min, ok)
	}
	maj, min, ok = parseGoVersion("go version go1.22.2 linux/amd64")
	if !ok || min != 22 {
		t.Fatalf("%d %d", maj, min)
	}
}

func TestStaleModelgen(t *testing.T) {
	root := t.TempDir()
	must := func(p string) string {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return full
	}
	model := must("internal/model/product.go")
	gen := must("internal/modelgen/product_gen.go")
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(gen, old, old); err != nil {
		t.Fatal(err)
	}
	got := staleModelgen(root)
	if len(got) != 1 || got[0] != model {
		t.Fatalf("want [%s], got %v", model, got)
	}
	now := time.Now().Add(time.Minute)
	_ = os.Chtimes(gen, now, now)
	if got := staleModelgen(root); len(got) != 0 {
		t.Fatalf("fresh modelgen must not be stale: %v", got)
	}
}
