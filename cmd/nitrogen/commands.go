package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var validProjectName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`) // kept for legacy tests; prefer validateProjectName

func cmdNew(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: nitrogen new <project-name> [--basic|--microservice|--worker|--fleet] [--module <path>] [--services a,b] [--nitro-path dir]")
		os.Exit(1)
	}

	name := args[0]
	if strings.HasPrefix(name, "-") {
		fmt.Fprintf(os.Stderr, "nitrogen: invalid project name %q (looks like a flag)\n", name)
		fmt.Fprintln(os.Stderr, "usage: nitrogen new <project-name> [--basic|--microservice|--worker|--fleet] [--module <path>]")
		os.Exit(1)
	}
	if err := validateProjectName(name); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}

	o, ok := parseNewArgs(args)
	if !ok {
		os.Exit(1)
	}
	o.module = normalizeModulePath(o.module)
	o.nitroPath = resolveNitroPath(o.nitroPath)

	if o.tier == "fleet" {
		cmdNewFleet(o)
		return
	}

	if err := os.MkdirAll(name, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: failed to create directory: %v\n", err)
		os.Exit(1)
	}

	files := projectTemplates(name, o.module, o.tier)
	if err := writeProjectFiles(name, files); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	if err := appendNitroReplace(filepath.Join(name, "go.mod"), o.nitroPath); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: replace: %v\n", err)
	}

	fmt.Printf("nitrogen: project %s created (%s)\n", name, o.tier)
	initAiRules(name)
	fmt.Printf("  cd %s\n", name)
	fmt.Println("  go mod tidy")
	switch o.tier {
	case "basic":
		fmt.Println("  nitrogen dev")
	case "worker":
		fmt.Println("  go run ./cmd/worker")
	default:
		fmt.Println("  go run ./cmd/server")
	}
}

func cmdDev(args []string) {
	if _, err := exec.LookPath("air"); err == nil {
		fmt.Println("nitrogen: starting with air (hot reload)")
		cmd := exec.Command("air")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("nitrogen: tip — install 'air' for hot reload: go install github.com/air-verse/air@latest")
	cmd := exec.Command("go", "run", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
}

// cmdBuild compiles the app: ./cmd/server in the CA layout (else the module
// root), stripped of symbol/DWARF tables (-s -w, ~30% smaller) and with
// -trimpath; --debug keeps them for debuggers.
func cmdBuild(args []string) {
	pkg, output := ".", "bin/app"
	if _, err := os.Stat("cmd/server/main.go"); err == nil {
		pkg, output = "./cmd/server", "bin/server"
	}
	debug := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "--debug":
			debug = true
		}
	}

	buildArgs := []string{"build", "-o", output}
	if !debug {
		buildArgs = append(buildArgs, "-trimpath", "-ldflags=-s -w")
	}
	cmd := exec.Command("go", append(buildArgs, pkg)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: build failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("nitrogen: binary built at %s\n", output)
}

func cmdMigrate(args []string) {
	// Check if goose is available (popular Go migration tool)
	if _, err := exec.LookPath("goose"); err == nil {
		cmd := exec.Command("goose", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: goose failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Fallback: look for custom migration binary or script
	if _, err := os.Stat("./bin/migrate"); err == nil {
		cmd := exec.Command("./bin/migrate", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: ./bin/migrate failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Check for a migrations directory
	if _, err := os.Stat("./migrations"); err == nil {
		fmt.Println("nitrogen: migrations directory found")
		fmt.Println("nitrogen: install goose to run migrations:")
		fmt.Println("  go install github.com/pressly/goose/v3/cmd/goose@latest")
		fmt.Println("  nitrogen migrate up")
		return
	}

	fmt.Println("nitrogen: no migration tool or migrations directory found")
	fmt.Println("nitrogen: to get started:")
	fmt.Println("  mkdir migrations")
	fmt.Println("  go install github.com/pressly/goose/v3/cmd/goose@latest")
	fmt.Println("  nitrogen migrate create add_users_table sql")
}

func cmdDoctor(args []string) {
	build := true
	for _, a := range args {
		switch a {
		case "--db":
			cmdDoctorDB()
			return
		case "--no-build":
			build = false
		}
	}

	fail := false
	var goVersionOut string

	checks := []struct {
		name     string
		cmd      string
		optional bool
	}{
		{"go", "go version || /bin/go version || /usr/bin/go version", false},
		{"git", "git --version", false},
		{"govulncheck", "govulncheck --version", true},
		{"goose", "goose --version", true},
		{"docker", "docker --version", true},
		{"curl", "curl --version", true},
		{"air", "air -v", true},
	}

	for _, check := range checks {
		out, err := exec.Command("sh", "-c", check.cmd).Output()
		if err != nil {
			if check.optional {
				fmt.Printf("  ⚡ %s: not found (optional)\n", check.name)
			} else {
				fmt.Printf("  ✗ %s: not found\n", check.name)
				fail = true
			}
		} else {
			firstLine := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
			fmt.Printf("  ✓ %s: %s\n", check.name, firstLine)
			if check.name == "go" {
				goVersionOut = firstLine
			}
		}
	}

	// Module files
	fmt.Println()
	var goMod string
	if b, err := os.ReadFile("go.mod"); err == nil {
		fmt.Println("  ✓ go.mod found")
		goMod = string(b)
	} else {
		fmt.Println("  ✗ go.mod not found (not a Go module)")
		fail = true
	}

	if goVersionOut != "" && goMod != "" {
		checkGoForSQL(goVersionOut, goMod)
	}

	// GOPATH
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = os.Getenv("HOME") + "/go"
	}
	fmt.Printf("  GOPATH: %s\n", gopath)

	if fail {
		fmt.Println("\nnitrogen: some required tools are missing")
		os.Exit(1)
	}
	enrichDoctor()
	warnStaleTableNameGen()
	warnStaleModelgen()
	if build && goMod != "" && !checkBuild() {
		fmt.Println("\nnitrogen: the project does not compile")
		os.Exit(1)
	}
	fmt.Println("\nnitrogen: all checks passed")
	fmt.Println("nitrogen: tip — nitrogen doctor --db  to ping SQL")
}

// checkBuild compiles the module (go vet also type-checks tests), so doctor
// never reports "all checks passed" for a project that does not build.
func checkBuild() bool {
	fmt.Println("Build:")
	ok := true
	for _, step := range [][]string{{"build", "./..."}, {"vet", "./..."}} {
		out, err := exec.Command("go", step...).CombinedOutput()
		if err == nil {
			fmt.Printf("  ✓ go %s\n", strings.Join(step, " "))
			continue
		}
		ok = false
		fmt.Printf("  ✗ go %s\n", strings.Join(step, " "))
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 12 {
			lines = append(lines[:12], fmt.Sprintf("… (%d more lines)", len(lines)-12))
		}
		for _, l := range lines {
			fmt.Printf("      %s\n", l)
		}
		if strings.Contains(string(out), "go mod tidy") || strings.Contains(string(out), "no required module provides") {
			fmt.Println("  tip — run: go mod tidy")
		}
		break // vet repeats build errors
	}
	return ok
}

func cmdBenchmark(args []string) {
	fmt.Println("nitrogen: running benchmarks...")
	cmd := exec.Command("go", "test", "-bench=.", "-benchmem", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: benchmarks failed: %v\n", err)
		os.Exit(1)
	}
}

func cmdAudit(args []string) {
	fmt.Println("nitrogen: checking for vulnerabilities...")

	// Check if govulncheck is installed
	if _, err := exec.LookPath("govulncheck"); err != nil {
		fmt.Println("nitrogen: govulncheck not found, installing...")
		install := exec.Command("go", "install", "golang.org/x/vuln/cmd/govulncheck@latest")
		install.Stdout = os.Stdout
		install.Stderr = os.Stderr
		if err := install.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: failed to install govulncheck: %v\n", err)
			fmt.Println("nitrogen: you can install it manually: go install golang.org/x/vuln/cmd/govulncheck@latest")
			os.Exit(1)
		}
	}

	cmd := exec.Command("govulncheck", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: vulnerabilities found\n")
		os.Exit(1)
	}
	fmt.Println("nitrogen: no known vulnerabilities found")
}
