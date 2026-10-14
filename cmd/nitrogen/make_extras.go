package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func cmdMakeSeeder(args []string) {
	name, _, _, _, _ := parseMakeNameArgs(args)
	if name == "" {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:seeder <Name>")
		os.Exit(1)
	}
	names, err := normalizeResourceName(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	dir := "internal/seed"
	if !hasInternalLayout() {
		dir = "seed"
	}
	mod := modulePath()
	path := filepath.Join(dir, names.File+"_seeder.go")
	pkg := "seed"
	var body string
	if hasSQLInstalled() && hasInternalLayout() && mod != "" {
		body = fmt.Sprintf(`package %s

import (
	"context"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"

	"%s/internal/model"
)

// Seed%s inserts a sample row (run via: nitrogen seed).
func Seed%s(ctx context.Context, db *sqladapter.DB) error {
	store := sqladapter.StoreOf[model.%s](db, %q)
	_, err := store.CreateOne(ctx, &model.%s{})
	return err
}
`, pkg, mod, names.Type, names.Type, names.Type, names.Table, names.Type)
	} else if hasInternalLayout() && mod != "" && dirExists(filepath.Join("internal", "usecase", names.Package)) {
		uc := names.Package
		body = fmt.Sprintf(`package %s

import (
	"context"

	"%s/internal/model"
	%suc "%s/internal/usecase/%s"
)

// Seed%s inserts a sample row (call from cmd/seed or bootstrap).
func Seed%s(ctx context.Context, repo %suc.Repository) error {
	_, err := repo.Create(ctx, &model.%s{})
	return err
}
`, pkg, mod, uc, mod, uc, names.Type, names.Type, uc, names.Type)
	} else {
		body = fmt.Sprintf(`package %s

import "context"

// Seed%s is a stub — install SQL (nitrogen install …) for Store-based seeding.
func Seed%s(ctx context.Context) error {
	return nil
}
`, pkg, names.Type, names.Type)
	}
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
	if hasSQLInstalled() {
		ensureSeedCmdFor(names.Type)
	}
}

func cmdMakeFactory(args []string) {
	name, _, _, _, _ := parseMakeNameArgs(args)
	if name == "" {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:factory <Name>")
		os.Exit(1)
	}
	dir := "internal/factory"
	if !hasInternalLayout() {
		dir = "factory"
	}
	mod := modulePath()
	path := filepath.Join(dir, strings.ToLower(name)+"_factory.go")
	var body string
	if hasInternalLayout() && mod != "" {
		body = fmt.Sprintf(`package factory

import "%s/internal/model"

// %s returns a %s for tests. Set required fields before Create.
func %s() model.%s {
	return model.%s{}
}
`, mod, name, name, name, name, name)
	} else {
		body = fmt.Sprintf(`package factory

func %s() %s {
	return %s{}
}
`, name, name, name)
	}
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
}

func cmdMakeMiddleware(args []string) {
	name := ""
	if len(args) > 0 {
		name = exportIdent(args[0])
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:middleware <Name>")
		os.Exit(1)
	}
	dir := "internal/middleware"
	if !hasInternalLayout() {
		dir = "middleware"
	}
	path := filepath.Join(dir, toSnake(name)+".go")
	body := fmt.Sprintf(`package middleware

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

// %s is HTTP middleware. Register with app.Use(%s).
// For JWT, prefer nitro.JWTMiddleware / nitro.RequireRoles instead of a custom parser.
func %s(c *nitro.Ctx) error {
	return c.Next()
}
`, name, name, name)
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
}

func cmdMakeJob(args []string) {
	name := ""
	if len(args) > 0 {
		name = exportIdent(args[0])
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:job <Name>")
		os.Exit(1)
	}
	dir := "internal/job"
	if !hasInternalLayout() {
		dir = "job"
	}
	path := filepath.Join(dir, toSnake(name)+".go")
	body := fmt.Sprintf(`package job

import "context"

// %s is a background job. Schedule with app.Schedule("1h", %s).
func %s(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
`, name, name, name)
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
}

func cmdMakeTest(args []string) {
	name, _, _, _, _ := parseMakeNameArgs(args)
	if name == "" {
		fmt.Fprintln(os.Stderr, "usage: nitrogen make:test <Name>")
		os.Exit(1)
	}
	names, err := normalizeResourceName(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	dir := "internal/handler"
	if !hasInternalLayout() {
		dir = "."
	}
	path := filepath.Join(dir, strings.ToLower(names.Type)+"_test.go")
	pkg := "handler"
	if !hasInternalLayout() {
		pkg = "main"
	}
	body := renderMakeTestFile(pkg, names)
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
}

func renderMakeTestFile(pkg string, names resourceNames) string {
	if hasSQLInstalled() && hasInternalLayout() && modulePath() != "" {
		mod := modulePath()
		return fmt.Sprintf(`package %s_test

import (
	"context"
	"path/filepath"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"%s/internal/handler"
	"%s/internal/model"
)

func Test%sHealth(t *testing.T) {
	app := nitro.New()
	app.Get("/health", handler.Health)
	tc := nitro.NewTestClient(app)
	resp := tc.Get(t, "/health")
	tc.AssertStatus(t, resp, 200)
}

func Test%sStore_OpenTest(t *testing.T) {
	ctx := context.Background()
	mig := filepath.Join("..", "..", "migrations")
	db, err := sqladapter.OpenTestMigrate(ctx, mig)
	if err != nil {
		t.Fatalf("OpenTestMigrate: %%v (run migrate:make first)", err)
	}
	defer db.Close()
	store := sqladapter.StoreOf[model.%s](db, %q)
	got, err := store.CreateOne(ctx, &model.%s{})
	if err != nil {
		t.Fatalf("CreateOne: %%v", err)
	}
	if got.ID.IsZero() {
		t.Fatal("expected generated id")
	}
}
`, pkg, mod, mod, names.Type, names.Type, names.Type, names.Table, names.Type)
	}
	return fmt.Sprintf(`package %s

import (
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func Test%sHealth(t *testing.T) {
	app := nitro.New()
	app.Get("/health", func(c *nitro.Ctx) error {
		return c.OK(nitro.Map{"status": "ok"})
	})
	tc := nitro.NewTestClient(app)
	resp := tc.Get(t, "/health")
	tc.AssertStatus(t, resp, 200)
}
`, pkg, names.Type)
}

func cmdModelShow(args []string) {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "usage: nitrogen model:show <Name|path.go>")
		os.Exit(1)
	}
	path := name
	if !strings.HasSuffix(path, ".go") {
		modelDir, _, _, _ := resourceDirs(exportIdent(name))
		path = filepath.Join(modelDir, strings.ToLower(exportIdent(name))+".go")
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Model file: %s\nPackage: %s\n\n", path, f.Name.Name)
	for _, d := range f.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, s := range gen.Specs {
			ts, ok := s.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			fmt.Printf("type %s struct {\n", ts.Name.Name)
			if st.Fields != nil {
				for _, field := range st.Fields.List {
					names := []string{}
					for _, n := range field.Names {
						names = append(names, n.Name)
					}
					tag := ""
					if field.Tag != nil {
						tag = field.Tag.Value
					}
					typeStr := exprString(field.Type)
					if len(names) == 0 {
						fmt.Printf("  %s %s\n", typeStr, tag)
					} else {
						fmt.Printf("  %s %s %s\n", strings.Join(names, ", "), typeStr, tag)
					}
				}
			}
			fmt.Println("}")
		}
	}
}

func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.ArrayType:
		return "[]" + exprString(t.Elt)
	default:
		return fmt.Sprintf("%T", e)
	}
}

func enrichDoctor() {
	fmt.Println("\nProject layout:")
	checks := []string{"go.mod", "internal", "internal/handler", "internal/model", "internal/usecase", "internal/repository", "migrations", "cmd/server", ".env", ".env.example"}
	for _, c := range checks {
		if _, err := os.Stat(c); err == nil {
			fmt.Printf("  ✓ %s\n", c)
		} else {
			fmt.Printf("  · %s (missing)\n", c)
		}
	}
	optional := []string{"internal/seed", "internal/factory", "internal/middleware", "internal/job"}
	fmt.Println("Optional (via make:*):")
	for _, c := range optional {
		if _, err := os.Stat(c); err == nil {
			fmt.Printf("  ✓ %s\n", c)
		} else {
			fmt.Printf("  · %s\n", c)
		}
	}
	fmt.Println("AI agent rules:")
	for _, c := range []string{"AGENTS.md", ".cursor/rules/nitro.mdc", ".cursor/prompts/nitro-blinq.md"} {
		if _, err := os.Stat(c); err == nil {
			fmt.Printf("  ✓ %s\n", c)
		} else {
			fmt.Printf("  · %s (missing — nitrogen ai:init)\n", c)
		}
	}
	if _, err := os.Stat("air"); err == nil {
		fmt.Println("  ✓ air")
	}
	b, err := os.ReadFile("internal/config/config.go")
	if err != nil {
		b, err = os.ReadFile("config/config.go")
	}
	if err == nil && strings.Contains(string(b), "type DBConfig struct") {
		fmt.Println("SQL:")
		if _, err := os.Stat("cmd/server/main.go"); err == nil {
			b, _ := os.ReadFile("cmd/server/main.go")
			if strings.Contains(string(b), "sqladapter.OpenEnv") {
				fmt.Println("  ✓ cmd/server opens DB via sqladapter.OpenEnv")
			} else {
				fmt.Println("  · cmd/server missing sqladapter.OpenEnv — nitrogen install postgresql|mysql|sqlite")
			}
		} else {
			fmt.Println("  · cmd/server/main.go missing")
		}
		gm, _ := os.ReadFile("go.mod")
		if strings.Contains(string(gm), "data/adapter/sql") {
			fmt.Println("  ✓ go.mod sql adapter")
		} else {
			fmt.Println("  · go.mod missing data/adapter/sql replace")
		}
		if os.Getenv("NITRO_DB_DRIVER") != "" || dotenvValue("NITRO_DB_DRIVER") != "" {
			fmt.Println("  ✓ NITRO_DB_DRIVER")
		} else {
			fmt.Println("  · NITRO_DB_DRIVER (set in .env via nitrogen install)")
		}
	}
	if rb, err := os.ReadFile("internal/handler/routes.go"); err == nil {
		if strings.Contains(string(rb), "Register(") {
			fmt.Println("  ✓ internal/handler/routes.go Register")
		}
	}
	doctorBLinqCompliance()
}

// doctorBLinqCompliance warns when alternate ORMs appear without an explicit
// BLinq opt-out, or when SQL is installed without modelgen / StoreOf.
func doctorBLinqCompliance() {
	fmt.Println("BLinq (default data access):")
	optOut := hasBLinqOptOut()
	alt := findAlternateORMImports(".")
	if len(alt) > 0 {
		if optOut {
			fmt.Printf("  ✓ alternate ORM present with BLinq opt-out: %s\n", strings.Join(uniqueSorted(alt), ", "))
		} else {
			fmt.Printf("  ⚠ ORM not opted out — BLinq is the default (found %s)\n", strings.Join(uniqueSorted(alt), ", "))
			fmt.Println("    tip: switch to StoreOf + modelgen, or document opt-out (e.g. \"skip BLinq\" in AGENTS.md)")
		}
	} else if optOut {
		fmt.Println("  · BLinq opt-out documented (no alternate ORM imports found)")
	} else {
		fmt.Println("  ✓ no GORM/sqlx/sqlc imports (BLinq default)")
	}

	sqlInstalled := false
	if b, err := os.ReadFile("internal/config/config.go"); err == nil && strings.Contains(string(b), "type DBConfig struct") {
		sqlInstalled = true
	} else if b, err := os.ReadFile("config/config.go"); err == nil && strings.Contains(string(b), "type DBConfig struct") {
		sqlInstalled = true
	}
	if !sqlInstalled {
		return
	}
	hasModelgen := dirExists("internal/modelgen") || dirExists("modelgen")
	hasStoreOf := projectContains("StoreOf[") || projectContains("sqladapter.StoreOf")
	if hasModelgen {
		fmt.Println("  ✓ modelgen package present")
	} else {
		fmt.Println("  · modelgen missing — nitrogen make <model>.go --data")
	}
	if hasStoreOf {
		fmt.Println("  ✓ StoreOf usage found")
	} else if !optOut && len(alt) == 0 {
		fmt.Println("  · StoreOf not found — prefer sqladapter.StoreOf[T](db, \"table\")")
	}
}

func hasBLinqOptOut() bool {
	markers := []string{
		"skip blinq",
		"nitro:skip-blinq",
		"nitro_skip_blinq",
		"opts out of blinq",
		"opted out of blinq",
		"nitro:orm=",
	}
	checkFiles := []string{
		"AGENTS.md", "AI.md", "README.md", "CLAUDE.md",
		".nitro-skip-blinq", ".nitro/skip-blinq",
	}
	for _, f := range checkFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		low := strings.ToLower(string(b))
		for _, m := range markers {
			if strings.Contains(low, m) {
				return true
			}
		}
		// bare marker file
		if strings.Contains(f, "skip-blinq") {
			return true
		}
	}
	return projectContains("//nitro:orm=") || projectContains("// nitro:orm=")
}

func findAlternateORMImports(root string) []string {
	var found []string
	if _, err := os.Stat(filepath.Join(root, "sqlc.yaml")); err == nil {
		found = append(found, "sqlc")
	}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == ".git" || base == "node_modules" || base == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		s := string(b)
		switch {
		case strings.Contains(s, "gorm.io/gorm") || strings.Contains(s, "\"gorm.io/"):
			found = append(found, "gorm")
		case strings.Contains(s, "github.com/jmoiron/sqlx"):
			found = append(found, "sqlx")
		case strings.Contains(s, "github.com/sqlc-dev") ||
			strings.Contains(s, "sqlc.yaml") ||
			(strings.Contains(s, "Code generated by sqlc") && strings.Contains(s, "DO NOT EDIT")):
			found = append(found, "sqlc")
		}
		return nil
	})
	return found
}

func projectContains(substr string) bool {
	found := false
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || found {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(b), substr) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func uniqueSorted(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
