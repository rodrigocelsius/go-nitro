package main

import (
	"os"
	"testing"
)

// doctor must not report "all checks passed" for a project that does not compile.
func TestDoctorCheckBuild(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "")
	if err := os.WriteFile("go.mod", []byte("module example.com/x\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("main.go", []byte("package main\n\nfunc main() { undefinedCall() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if checkBuild() {
		t.Fatal("checkBuild passed on a project that does not compile")
	}
	if err := os.WriteFile("main.go", []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !checkBuild() {
		t.Fatal("checkBuild failed on a valid project")
	}
}
