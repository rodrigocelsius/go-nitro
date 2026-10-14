package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyAiInitCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	written, skipped, err := applyAiInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Fatalf("skipped=%v", skipped)
	}
	if len(written) != 5 {
		t.Fatalf("written=%v", written)
	}
	for _, rel := range aiInitRelFiles {
		b, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if len(b) == 0 {
			t.Fatalf("%s empty", rel)
		}
		s := string(b)
		if strings.Contains(s, "esta app") || strings.Contains(s, "Obrigatório") || strings.Contains(s, "português") {
			t.Errorf("%s: expected English consumer contract, got Portuguese: %s", rel, s[:min(80, len(s))])
		}
	}
	agents, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(agents), "nitro.Parse") {
		t.Fatal("AGENTS.md missing Parse contract")
	}
	if !strings.Contains(string(agents), "DEFAULT DATA ACCESS") {
		t.Fatal("AGENTS.md missing DEFAULT DATA ACCESS STOP")
	}
	prompt, _ := os.ReadFile(filepath.Join(dir, ".cursor/prompts/nitro-blinq.md"))
	if !strings.Contains(string(prompt), "BLinq only") {
		t.Fatal("nitro-blinq.md missing reminder bullets")
	}
	if !strings.Contains(string(prompt), "app.Resource") {
		t.Fatal("nitro-blinq.md missing Resource-first shape")
	}
}

func TestApplyAiInitIdempotentWithoutForce(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := applyAiInit(dir, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(path, []byte("KEEP\n"), 0644); err != nil {
		t.Fatal(err)
	}
	written, skipped, err := applyAiInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 0 {
		t.Fatalf("wrote on second pass: %v", written)
	}
	if len(skipped) != 5 {
		t.Fatalf("skipped=%v", skipped)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "KEEP\n" {
		t.Fatalf("file overwritten without --force: %q", b)
	}
}

func TestApplyAiInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := applyAiInit(dir, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(path, []byte("KEEP\n"), 0644); err != nil {
		t.Fatal(err)
	}
	written, skipped, err := applyAiInit(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Fatalf("skipped=%v", skipped)
	}
	if len(written) != 5 {
		t.Fatalf("written=%v", written)
	}
	b, _ := os.ReadFile(path)
	if string(b) == "KEEP\n" {
		t.Fatal("expected --force to overwrite AGENTS.md")
	}
	if !strings.Contains(string(b), "BLinq") {
		t.Fatalf("overwritten AGENTS.md missing BLinq: %s", b[:min(120, len(b))])
	}
}

func TestParseAiInitArgs(t *testing.T) {
	o, err := parseAiInitArgs([]string{"--dir", "/tmp/app", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if o.dir != "/tmp/app" || !o.force {
		t.Fatalf("%+v", o)
	}
}
