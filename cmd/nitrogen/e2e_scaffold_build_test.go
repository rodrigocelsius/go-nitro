package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestE2E_ScaffoldManyResourcesBuilds runs the documented flow — new → install
// sqlite → several make:scaffold (CA and --resource, mixed) — and requires the
// generated app to compile. A second CA scaffold used to break the build
// (missing usecase import in routes.go, Register<Type> never wired in main).
func TestE2E_ScaffoldManyResourcesBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: runs go build on a generated app")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	nitroRoot := filepath.Clean(filepath.Join(cwd, "../.."))
	if _, err := os.Stat(filepath.Join(nitroRoot, "pkg")); err != nil {
		t.Skip("cannot locate nitro pkg for --nitro-path")
	}
	root := t.TempDir()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	cmdNew([]string{"shop", "--module", "example.com/shop", "--nitro-path", nitroRoot})
	if err := os.Chdir(filepath.Join(root, "shop")); err != nil {
		t.Fatal(err)
	}
	cmdInstall([]string{"sqlite"})
	cmdMakeScaffold([]string{"Order", "total:float64", "status:string"})
	cmdMakeScaffold([]string{"Customer", "name:string", "email:string"})
	cmdMakeScaffold([]string{"Category", "name:string", "--resource"})

	for _, args := range [][]string{
		{"mod", "tidy"},
		{"build", "./..."},
		{"vet", "./..."},
		{"test", "-count=1", "./..."},
	} {
		cmd := exec.Command("go", args...)
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off", "NITRO_OTEL=0", "NITRO_DB_QUIET=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			main, _ := os.ReadFile("cmd/server/main.go")
			routes, _ := os.ReadFile("internal/handler/routes.go")
			t.Fatalf("go %v: %v\n%s\n--- main.go\n%s\n--- routes.go\n%s", args, err, out, main, routes)
		}
	}
	main, _ := os.ReadFile("cmd/server/main.go")
	for _, call := range []string{"handler.Register(app, ", "handler.RegisterCustomer(app, ", "handler.RegisterCategory(app, "} {
		if !strings.Contains(string(main), call) {
			t.Errorf("main.go does not wire %s…\n%s", call, main)
		}
	}
}
