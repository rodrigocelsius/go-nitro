package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type upOpts struct {
	noDev       bool
	seed        bool
	migrateOnly bool
}

func parseUpArgs(args []string) upOpts {
	var o upOpts
	for _, a := range args {
		switch a {
		case "--no-dev":
			o.noDev = true
		case "--seed":
			o.seed = true
		case "--migrate-only":
			o.migrateOnly = true
			o.noDev = true
		}
	}
	return o
}

func cmdUp(args []string) {
	o := parseUpArgs(args)
	loadDotenvIntoEnv()

	if composeFile := findComposeFile(); composeFile != "" {
		fmt.Printf("nitrogen: docker compose up -d (%s)\n", composeFile)
		if err := runCompose(composeFile, "up", "-d"); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: compose up failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "nitrogen: tip — install Docker or start the stack manually")
			os.Exit(1)
		}
		if hasSQLEnv() {
			fmt.Println("nitrogen: waiting for database…")
			if err := waitForDB(90 * time.Second); err != nil {
				fmt.Fprintf(os.Stderr, "nitrogen: database not ready: %v\n", err)
				fmt.Fprintln(os.Stderr, "nitrogen: tip — nitrogen doctor --db")
				os.Exit(1)
			}
			fmt.Println("nitrogen: database ready")
		}
	} else if hasSQLEnv() {
		fmt.Println("nitrogen: no docker-compose.yml — skipping compose")
		if err := pingDBOnce(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: database unreachable: %v\n", err)
			fmt.Fprintln(os.Stderr, "nitrogen: tip — nitrogen install postgresql|sqlite  or  start your DB")
			os.Exit(1)
		}
	}

	if err := runMigrateUpBestEffort(); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: migrate: %v\n", err)
	}

	if o.seed {
		cmdSeed(nil)
	}

	if o.migrateOnly || o.noDev {
		fmt.Println("nitrogen: up complete (--no-dev / --migrate-only)")
		return
	}

	runAppDev()
}

func cmdDown(args []string) {
	volumes := false
	for _, a := range args {
		if a == "--volumes" || a == "-v" {
			volumes = true
		}
	}
	composeFile := findComposeFile()
	if composeFile == "" {
		fmt.Println("nitrogen: no docker-compose.yml found")
		return
	}
	argsOut := []string{"down"}
	if volumes {
		argsOut = append(argsOut, "--volumes")
	}
	fmt.Printf("nitrogen: docker compose %s (%s)\n", strings.Join(argsOut, " "), composeFile)
	if err := runCompose(composeFile, argsOut...); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: compose down failed: %v\n", err)
		os.Exit(1)
	}
}

func findComposeFile() string {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return ""
}

func runCompose(file string, args ...string) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found")
	}
	cmdArgs := append([]string{"compose", "-f", file}, args...)
	cmd := exec.Command("docker", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func hasSQLEnv() bool {
	return envOrDotenv("NITRO_DB_DRIVER") != "" ||
		envOrDotenv("DATABASE_URL") != "" ||
		envOrDotenv("DB_DSN") != "" ||
		envOrDotenv("POSTGRES_DSN") != ""
}

func envOrDotenv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return dotenvValue(key)
}

// loadDotenvIntoEnv sets missing keys from .env / .env.local / .env.example (does not override existing env).
func loadDotenvIntoEnv() {
	for _, file := range []string{".env", ".env.local", ".env.example"} {
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			v = strings.Trim(v, `"'`)
			if k == "" {
				continue
			}
			if os.Getenv(k) == "" {
				_ = os.Setenv(k, v)
			}
		}
	}
}

func waitForDB(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		last = pingDBOnce()
		if last == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	if last == nil {
		last = fmt.Errorf("timeout")
	}
	return last
}

func pingDBOnce() error {
	return runProjectDBCheck()
}

func runMigrateUpBestEffort() error {
	if _, err := os.Stat("migrations"); err != nil {
		return nil
	}
	if _, err := exec.LookPath("goose"); err == nil {
		driver := envOrDotenv("NITRO_DB_DRIVER")
		dsn := envOrDotenv("DATABASE_URL")
		if dsn == "" {
			dsn = envOrDotenv("DB_DSN")
		}
		if dsn == "" {
			dsn = envOrDotenv("POSTGRES_DSN")
		}
		// Apps typically call MigrateUp on Listen; goose CLI needs driver-specific flags.
		if driver != "" && dsn != "" {
			fmt.Println("nitrogen: tip — prefer MigrateUp in main; attempting goose when configured")
		}
	}
	// Prefer in-app migrate via the same go-run check helper with -migrate flag.
	return runProjectMigrate()
}

func runAppDev() {
	if _, err := os.Stat("cmd/server"); err == nil {
		if _, err := exec.LookPath("air"); err == nil {
			if _, err := os.Stat(".air.toml"); err == nil {
				fmt.Println("nitrogen: starting with air (hot reload)")
				cmd := exec.Command("air")
				cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
				if err := cmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
					os.Exit(1)
				}
				return
			}
		}
		fmt.Println("nitrogen: go run ./cmd/server")
		cmd := exec.Command("go", "run", "./cmd/server")
		cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if _, err := os.Stat("cmd/worker"); err == nil {
		fmt.Println("nitrogen: go run ./cmd/worker")
		cmd := exec.Command("go", "run", "./cmd/worker")
		cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
			os.Exit(1)
		}
		return
	}
	cmdDev(nil)
}

const doctorDBMain = `package main

import (
	"context"
	"fmt"
	"os"
	"time"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
)

func main() {
	_ = os.Setenv("NITRO_DB_QUIET", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	migrate := false
	for _, a := range os.Args[1:] {
		if a == "-migrate" {
			migrate = true
		}
	}
	db, err := sqladapter.OpenEnv(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping: %v\n", err)
		os.Exit(1)
	}
	if migrate {
		if err := db.MigrateUp(ctx); err != nil {
			// missing/empty migrations dir is ok for ping-only flows
			fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("migrate ok")
		return
	}
	fmt.Println("ok")
}
`

func ensureDoctorDBHelper() (string, error) {
	dir := filepath.Join(".nitro", "doctor-db")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "main.go")
	if err := writeFileFmt(path, []byte(doctorDBMain), 0644); err != nil {
		return "", err
	}
	// Ensure .nitro is gitignored hint once
	_ = ensureNitroGitignore()
	return dir, nil
}

func ensureNitroGitignore() error {
	const line = ".nitro/"
	b, err := os.ReadFile(".gitignore")
	if err != nil {
		return nil
	}
	if strings.Contains(string(b), line) {
		return nil
	}
	f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil
	}
	defer f.Close()
	_, _ = f.WriteString("\n# nitrogen up/doctor helper\n.nitro/\n")
	return nil
}

func runProjectDBCheck() error {
	gm, err := os.ReadFile("go.mod")
	if err != nil {
		return fmt.Errorf("go.mod required (run from app root)")
	}
	if !strings.Contains(string(gm), "data/adapter/sql") {
		return fmt.Errorf("go.mod missing data/adapter/sql — run: nitrogen install postgresql|mysql|sqlite")
	}
	dir, err := ensureDoctorDBHelper()
	if err != nil {
		return err
	}
	cmd := exec.Command("go", "run", "./"+filepath.ToSlash(dir))
	cmd.Env = append(os.Environ(), "NITRO_DB_QUIET=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func runProjectMigrate() error {
	gm, err := os.ReadFile("go.mod")
	if err != nil {
		return nil
	}
	if !strings.Contains(string(gm), "data/adapter/sql") {
		return nil
	}
	if _, err := os.Stat("migrations"); err != nil {
		return nil
	}
	// Skip if only .gitkeep
	entries, _ := os.ReadDir("migrations")
	hasSQL := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			hasSQL = true
			break
		}
	}
	if !hasSQL {
		fmt.Println("nitrogen: no .sql migrations yet (skip MigrateUp)")
		return nil
	}
	dir, err := ensureDoctorDBHelper()
	if err != nil {
		return err
	}
	fmt.Println("nitrogen: MigrateUp via sqladapter…")
	cmd := exec.Command("go", "run", "./"+filepath.ToSlash(dir), "-migrate")
	cmd.Env = append(os.Environ(), "NITRO_DB_QUIET=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cmdDoctorDB() {
	loadDotenvIntoEnv()
	fail := false

	driver := envOrDotenv("NITRO_DB_DRIVER")
	dsn := envOrDotenv("DATABASE_URL")
	if dsn == "" {
		dsn = envOrDotenv("DB_DSN")
	}
	if dsn == "" {
		dsn = envOrDotenv("POSTGRES_DSN")
	}

	fmt.Println("Database:")
	if driver == "" {
		fmt.Println("  ✗ NITRO_DB_DRIVER missing — run: nitrogen install postgresql|mysql|mariadb|sqlserver|sqlite")
		fail = true
	} else {
		fmt.Printf("  ✓ NITRO_DB_DRIVER=%s\n", driver)
	}
	if dsn == "" && driver != "sqlite" {
		fmt.Println("  ✗ DATABASE_URL / DB_DSN missing")
		fail = true
	} else if dsn != "" {
		fmt.Println("  ✓ DSN present")
	} else {
		fmt.Println("  · sqlite without DSN (default file path ok)")
	}

	if compose := findComposeFile(); compose != "" {
		fmt.Printf("  ✓ %s\n", compose)
	} else {
		fmt.Println("  · no docker-compose.yml")
	}

	if fail {
		fmt.Println("\nnitrogen: fix env, then retry: nitrogen doctor --db")
		os.Exit(1)
	}

	fmt.Println("  … ping")
	if err := runProjectDBCheck(); err != nil {
		fmt.Printf("  ✗ ping failed: %v\n", err)
		fmt.Println("nitrogen: tip — nitrogen up --migrate-only   or   docker compose up -d")
		os.Exit(1)
	}
	fmt.Println("  ✓ ping ok")

	// Optional: goose version table when migrations exist
	if _, err := os.Stat("migrations"); err == nil {
		fmt.Println("  · migrations/ present (app MigrateUp on Listen / nitrogen up)")
	}
	fmt.Println("\nnitrogen: database checks passed")
}

// parseGoVersion returns major.minor from "go version go1.25.1 linux/amd64".
func parseGoVersion(out string) (major, minor int, ok bool) {
	i := strings.Index(out, "go1.")
	if i < 0 {
		return 0, 0, false
	}
	rest := strings.TrimPrefix(out[i:], "go") // "1.25.1 ..."
	parts := strings.SplitN(rest, " ", 2)[0]
	segs := strings.Split(parts, ".")
	if len(segs) < 2 {
		return 0, 0, false
	}
	maj, err1 := strconv.Atoi(segs[0])
	min, err2 := strconv.Atoi(segs[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return maj, min, true
}

func checkGoForSQL(goVersionOut string, goMod string) {
	needs := strings.Contains(goMod, "data/adapter/sql") || strings.Contains(goMod, "data/adapter/pgx")
	if !needs {
		return
	}
	maj, min, ok := parseGoVersion(goVersionOut)
	if !ok {
		fmt.Println("  · Go version: could not parse (SQL adapters need Go 1.25+)")
		return
	}
	if maj > 1 || (maj == 1 && min >= 25) {
		fmt.Printf("  ✓ Go %d.%d (SQL adapters require 1.25+)\n", maj, min)
		return
	}
	fmt.Printf("  ✗ Go %d.%d — data/adapter/sql and pgx need Go 1.25+\n", maj, min)
}

func warnStaleTableNameGen() {
	var found []string
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == ".git" || base == "node_modules" || base == ".nitro" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), "_tablename_gen.go") {
			found = append(found, path)
		}
		return nil
	})
	if len(found) == 0 {
		return
	}
	fmt.Println("Stale codegen:")
	for _, f := range found {
		fmt.Printf("  · %s — remove and run: nitrogen make <model.go> --data\n", f)
	}
}

// staleModelgen lists model files that are newer than their generated DSL
// (<dir>/model/x.go → <dir>/modelgen/x_gen.go, or <dir>/x.go → <dir>/modelgen/x_gen.go).
func staleModelgen(root string) []string {
	var stale []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case "vendor", ".git", "node_modules", ".nitro", "modelgen":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_gen.go") {
			return nil
		}
		dir := filepath.Dir(path)
		base := strings.TrimSuffix(filepath.Base(path), ".go")
		candidates := []string{
			filepath.Join(filepath.Dir(dir), "modelgen", base+"_gen.go"), // internal/model → internal/modelgen
			filepath.Join(dir, "modelgen", base+"_gen.go"),               // productmodel → productmodel/modelgen
		}
		for _, gen := range candidates {
			if gi, err := os.Stat(gen); err == nil {
				if info.ModTime().After(gi.ModTime()) {
					stale = append(stale, path)
				}
				break
			}
		}
		return nil
	})
	return stale
}

func warnStaleModelgen() {
	stale := staleModelgen(".")
	if len(stale) == 0 {
		return
	}
	fmt.Println("Stale BLinq metamodel:")
	for _, f := range stale {
		fmt.Printf("  ⚠ %s changed after its modelgen — run: nitrogen make %s --data\n", f, f)
	}
}
