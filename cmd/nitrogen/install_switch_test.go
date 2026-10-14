package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertEnvVars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("NITRO_DB_DRIVER=postgresql\nFOO=bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := upsertEnvVars(path, map[string]string{
		"NITRO_DB_DRIVER": "sqlite",
		"DATABASE_URL":    "file:test.db",
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if !strings.Contains(src, "NITRO_DB_DRIVER=sqlite") {
		t.Fatalf("driver not upserted: %s", src)
	}
	if !strings.Contains(src, "FOO=bar") {
		t.Fatalf("preserved key missing: %s", src)
	}
	if !strings.Contains(src, "DATABASE_URL=file:test.db") {
		t.Fatalf("new key missing: %s", src)
	}
	if strings.Contains(src, "NITRO_DB_DRIVER=postgresql") {
		t.Fatalf("old driver still present: %s", src)
	}
}

func TestCommentComposeService(t *testing.T) {
	src := `services:
  postgres:
    image: postgres:16
    ports:
      - "5432:5432"
  redis:
    image: redis:7
volumes:
  data:
`
	out := commentComposeService(src, "postgres")
	if !strings.Contains(out, "#   postgres:") {
		t.Fatalf("service not commented:\n%s", out)
	}
	if !strings.Contains(out, "  redis:") {
		t.Fatalf("rival should stay active:\n%s", out)
	}
	if !strings.Contains(out, "volumes:") {
		t.Fatalf("volumes lost:\n%s", out)
	}
}

func TestWarnEngineSwitchNoOp(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// no .env — should not panic
	warnEngineSwitch("sqlite")
}
