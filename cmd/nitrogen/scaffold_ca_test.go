package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectSQLAndDialect(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if hasSQLInstalled() {
		t.Fatal("empty dir should not look like SQL")
	}
	if err := os.WriteFile(".env", []byte("NITRO_DB_DRIVER=mysql\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if dotenvValue("NITRO_DB_DRIVER") != "mysql" {
		t.Fatal(dotenvValue("NITRO_DB_DRIVER"))
	}
	_ = os.MkdirAll("internal/config", 0755)
	if err := os.WriteFile("internal/config/config.go", []byte("package config\ntype DBConfig struct{ DSN string }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !hasSQLInstalled() {
		t.Fatal("DBConfig should count as SQL installed")
	}
}

func TestScaffoldMemoryHandlers(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("go.mod", []byte("module example.com/app\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{
		"internal/model", "internal/usecase", "internal/handler", "internal/repository", "cmd/server",
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile("cmd/server/main.go", []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("internal/handler/routes.go", []byte("package handler\n\nimport nitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"\n\nfunc Register(app *nitro.App) {\n\tapp.Get(\"/health\", Health)\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("internal/handler/health.go", []byte("package handler\n\nimport nitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"\n\nfunc Health(c *nitro.Ctx) error { return c.OK(nitro.Map{\"status\":\"ok\"}) }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmdMakeScaffold([]string{"Product", "name:string", "price:float64", "active:bool"})

	h, err := os.ReadFile("internal/handler/product.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(h)
	if !strings.Contains(src, "nitro.Parse[model.Product]") {
		t.Fatalf("handler should Parse: %s", src)
	}
	if !strings.Contains(src, "c.Created") {
		t.Fatal("missing Created")
	}
	rt, err := os.ReadFile("internal/handler/routes.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rt), "Register(app *nitro.App, repo") {
		t.Fatalf("routes: %s", rt)
	}
	if !strings.Contains(string(rt), "/products") {
		t.Fatal("missing /products")
	}
	uc, err := os.ReadFile("internal/usecase/product/list.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(uc), "Filter") {
		t.Fatalf("list usecase: %s", uc)
	}
	if _, err := os.Stat("internal/repository/product.go"); err == nil {
		t.Fatal("memory-only scaffold should not write BLinq repo")
	}
}

func TestScaffoldMySQLWritesBlinqRepo(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("go.mod", []byte("module example.com/app\n\ngo 1.22\n\nreplace github.com/rodrigocelsius/go-nitro/pkg/v2 => /tmp/nope/pkg\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{
		"internal/model", "internal/usecase", "internal/handler", "internal/repository", "internal/config", "cmd/server",
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile("internal/config/config.go", []byte("package config\ntype DBConfig struct{ DSN string }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".env", []byte("NITRO_DB_DRIVER=mysql\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("cmd/server/main.go", []byte(`package main
import (
	"context"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
	"example.com/app/internal/handler"
)
func main() {
	db, _ := sqladapter.OpenEnv(context.Background())
	_ = db
	handler.Register(app)
}
var app *nitro.App
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("internal/handler/routes.go", []byte("package handler\n\nimport nitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"\n\nfunc Register(app *nitro.App) {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmdMakeScaffold([]string{"Product", "name:string", "price:float64", "active:bool"})

	repo, err := os.ReadFile("internal/repository/product.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repo), "sqladapter.Store") {
		t.Fatalf("expected sqladapter store, got %s", repo)
	}
	if strings.Contains(string(repo), "pgxadapter") {
		t.Fatalf("generated repository must not hardcode pgx: %s", repo)
	}
	mainb, _ := os.ReadFile("cmd/server/main.go")
	if !strings.Contains(string(mainb), "sqladapter.StoreFor[model.Product](db)") {
		t.Fatalf("main: %s", mainb)
	}
	if strings.Contains(string(mainb), "pgxadapter.Open") {
		t.Fatal("main should use platform/sqladapter, not pgxadapter")
	}
	if _, err := os.Stat("internal/platform/db.go"); err == nil {
		t.Fatal("platform/db.go should not be created for SQL+cache")
	}
	if _, err := os.Stat(filepath.Join("internal", "modelgen")); err != nil {
		t.Log("modelgen may be missing if codegen failed on empty struct tags — check logs")
	}
}

func TestRenderHandlerUsesParse(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("internal", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("go.mod", []byte("module example.com/app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fields := []fieldSpec{{Name: "Name", Type: "string"}, {Name: "Active", Type: "bool"}}
	s := renderHandlerFile("Product", "products", fields)
	for _, want := range []string{"nitro.Parse[model.Product]", "c.Int(\"page\"", "c.Query(\"q\")", "c.Query(\"active\")", "c.Query(\"sort\")"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
}

func TestScaffoldResourceMode(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("go.mod", []byte("module example.com/app\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{
		"internal/model", "internal/usecase", "internal/handler", "internal/repository", "cmd/server",
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile("cmd/server/main.go", []byte(`package main
import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
	"example.com/app/internal/handler"
)
func main() {
	app := nitro.New()
	handler.Register(app)
	_ = app.Listen(":8080")
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("internal/handler/health.go", []byte("package handler\n\nimport nitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"\n\nfunc Health(c *nitro.Ctx) error { return c.OK(nitro.Map{\"status\":\"ok\"}) }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("internal/handler/routes.go", []byte("package handler\n\nimport nitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"\n\nfunc Register(app *nitro.App) {\n\tapp.Get(\"/health\", Health)\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmdMakeScaffold([]string{"Product", "name:string", "price:float64", "active:bool", "--resource", "--with-seed"})

	if _, err := os.Stat("internal/usecase/product/list.go"); err == nil {
		t.Fatal("--resource must not write CA usecases")
	}
	if _, err := os.Stat("internal/handler/product.go"); err == nil {
		t.Fatal("--resource must not write CA handlers")
	}
	rt, err := os.ReadFile("internal/handler/routes.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(rt)
	if !strings.Contains(src, "app.Resource(&model.Product{}") {
		t.Fatalf("routes Resource: %s", src)
	}
	if !strings.Contains(src, `Param: "q"`) {
		t.Fatal("expected FilterRules q")
	}
	if _, err := os.Stat("internal/seed/product_seeder.go"); err != nil {
		t.Fatal("expected --with-seed to create seeder")
	}
	mainb, _ := os.ReadFile("cmd/server/main.go")
	if !strings.Contains(string(mainb), "NewMemoryRepo[model.Product]") {
		t.Fatalf("main memory Resource: %s", mainb)
	}
}

func TestParseScaffoldArgs(t *testing.T) {
	_, fields, o := parseScaffoldArgs([]string{"Product", "name:string", "--resource", "--with-factory"})
	if !o.resource || !o.withFactory || o.withSeed {
		t.Fatalf("%+v", o)
	}
	if len(fields) != 1 {
		t.Fatalf("fields %v", fields)
	}
}

func TestPreferResourceMode_BasicWithSQL(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile("go.mod", []byte("module example.com/basic\n"), 0644)
	// After `nitrogen install sqlite` on a --basic app, NITRO_DB_DRIVER is set.
	_ = os.WriteFile(".env", []byte("NITRO_DB_DRIVER=sqlite\n"), 0644)
	if !preferResourceMode(scaffoldOpts{}) {
		t.Fatalf("basic+SQL should prefer Resource (hasSQL=%v layout=%v)", hasSQLInstalled(), hasInternalLayout())
	}
	_ = os.MkdirAll("internal", 0755)
	if preferResourceMode(scaffoldOpts{}) {
		t.Fatal("CA layout must not auto Resource without --resource")
	}
	if !preferResourceMode(scaffoldOpts{resource: true}) {
		t.Fatal("--resource always wins")
	}
}

// make:usecase after make:scaffold must not rewrite the repository port
// (it used to drop Filter.Q and break the generated handler/repository).
func TestMakeUsecaseKeepsScaffoldRepositoryPort(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile("go.mod", []byte("module example.com/app\n\ngo 1.22\n"), 0644)
	_ = os.MkdirAll("internal/usecase/product", 0755)
	_ = os.MkdirAll("internal/model", 0755)
	_ = os.MkdirAll("internal/handler", 0755)
	port := "package product\n\ntype Filter struct {\n\tQ string\n}\n"
	_ = os.WriteFile("internal/usecase/product/repository.go", []byte(port), 0644)
	ensureUsecaseRepoInterface("Product", nil)
	b, _ := os.ReadFile("internal/usecase/product/repository.go")
	if string(b) != port {
		t.Fatalf("repository port rewritten:\n%s", b)
	}
}
