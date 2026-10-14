package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func cmdSeed(args []string) {
	_ = args
	loadDotenvIntoEnv()

	if _, err := os.Stat("cmd/seed"); err == nil {
		fmt.Println("nitrogen: go run ./cmd/seed")
		cmd := exec.Command("go", "run", "./cmd/seed")
		cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: seed failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	seedFile := "./seed.go"
	if _, err := os.Stat(seedFile); err == nil {
		fmt.Println("nitrogen: running seed.go...")
		cmd := exec.Command("go", "run", seedFile)
		cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: seed failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Auto-generate cmd/seed when seeders exist but runner is missing.
	if names := discoverSeedFuncs(); len(names) > 0 {
		fmt.Println("nitrogen: generating cmd/seed from internal/seed…")
		if err := writeSeedCmd(names); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
			os.Exit(1)
		}
		cmdSeed(nil)
		return
	}

	fmt.Println("nitrogen: no seeders found")
	fmt.Println("nitrogen: create with: nitrogen make:seeder Product")
	fmt.Println("nitrogen: then: nitrogen seed")
}

func discoverSeedFuncs() []string {
	var names []string
	for _, dir := range []string{"internal/seed", "seed"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				continue
			}
			for _, d := range f.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || fn.Name == nil {
					continue
				}
				if strings.HasPrefix(fn.Name.Name, "Seed") && fn.Name.IsExported() {
					names = append(names, fn.Name.Name)
				}
			}
		}
	}
	return uniqueStrings(names)
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func writeSeedCmd(seedFuncs []string) error {
	mod := modulePath()
	if mod == "" {
		return fmt.Errorf("go.mod module path required")
	}
	dir := "cmd/seed"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "main.go")

	var calls strings.Builder
	for _, fn := range seedFuncs {
		fmt.Fprintf(&calls, "\tif err := seed.%s(ctx, db); err != nil {\n\t\tlog.Fatalf(\"%%s: %%v\", %q, err)\n\t}\n\tlog.Println(%q)\n", fn, fn, fn+" ok")
	}

	body := fmt.Sprintf(`package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	"%s/internal/seed"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	_ = os.Setenv("NITRO_DB_QUIET", "1")
	db, err := sqladapter.OpenEnv(ctx)
	if err != nil {
		log.Fatalf("db: %%v (set NITRO_DB_DRIVER + DATABASE_URL)", err)
	}
	defer db.Close()
%s	log.Println("seed complete")
}
`, mod, calls.String())
	return writeFile(path, body)
}

func ensureSeedCmdFor(typeName string) {
	fn := "Seed" + typeName
	names := discoverSeedFuncs()
	found := false
	for _, n := range names {
		if n == fn {
			found = true
			break
		}
	}
	if !found {
		names = append(names, fn)
	}
	if err := writeSeedCmd(names); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: cmd/seed: %v\n", err)
		return
	}
	fmt.Println("nitrogen: wrote/updated cmd/seed/main.go")
}
