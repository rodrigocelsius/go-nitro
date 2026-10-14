package fasthttp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/valyala/fasthttp"
)

// TestStatic_RejectsSymlinkEscapingRoot covers V-31: a symlink placed
// inside the static root that points outside of it (e.g. to a sensitive
// file elsewhere on disk) must not be served, even though the request
// path itself never contains "..".
func TestStatic_RejectsSymlinkEscapingRoot(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()

	secretPath := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("top secret"), 0600); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(rootDir, "evil.txt")
	if err := os.Symlink(secretPath, linkPath); err != nil {
		t.Skipf("symlinks not supported in this environment: %v", err)
	}

	// A legitimate file inside root must still be served normally.
	if err := os.WriteFile(filepath.Join(rootDir, "ok.txt"), []byte("fine"), 0644); err != nil {
		t.Fatal(err)
	}

	app := New()
	app.Static("/", rootDir)

	evilCtx := &fasthttp.RequestCtx{}
	evilCtx.Request.SetRequestURI("/evil.txt")
	app.Handler()(evilCtx)
	if evilCtx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("expected 403 for symlink escaping root, got %d body=%q", evilCtx.Response.StatusCode(), evilCtx.Response.Body())
	}

	okCtx := &fasthttp.RequestCtx{}
	okCtx.Request.SetRequestURI("/ok.txt")
	app.Handler()(okCtx)
	if okCtx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200 for legitimate file, got %d", okCtx.Response.StatusCode())
	}
}
