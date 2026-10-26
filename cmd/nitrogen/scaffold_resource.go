package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	migrationgen "github.com/rodrigocelsius/go-nitro/codegen/migration/v2"
)

// scaffoldOpts extends make:scaffold beyond the shared parseMakeNameArgs flags.
type scaffoldOpts struct {
	resource    bool
	withSeed    bool
	withFactory bool
	force       bool
}

func parseScaffoldArgs(args []string) (name string, fields []fieldSpec, o scaffoldOpts) {
	var resource bool
	var show bool
	var rest []string
	name, resource, fields, show, rest = parseMakeNameArgs(args)
	_ = show
	o.resource = resource
	for _, a := range rest {
		switch a {
		case "--with-seed":
			o.withSeed = true
		case "--with-factory":
			o.withFactory = true
		case "--force":
			o.force = true
		case "--resource", "-r":
			o.resource = true
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "nitrogen: unknown flag %s\n", a)
				os.Exit(1)
			}
		}
	}
	return name, fields, o
}

// preferResourceMode: explicit --resource, or basic (no internal/) app with SQL installed.
func preferResourceMode(o scaffoldOpts) bool {
	if o.resource {
		return true
	}
	return !hasInternalLayout() && hasSQLInstalled()
}

func cmdMakeScaffoldResource(names resourceNames, fields []fieldSpec, pkgModel string, force bool) {
	modelDir, _, handlerDir, _ := resourceDirs(names.Package)
	modelPath := filepath.Join(modelDir, names.File+".go")
	if wrote, err := writeFileMaybe(modelPath, renderModelFilePkg(pkgModel, names.Type, names.Table, fields), force); err != nil {
		fatalWrite(err)
	} else if wrote {
		fmt.Printf("nitrogen: wrote %s (type %s, table %s) [--resource]\n", modelPath, names.Type, names.Table)
	} else {
		fmt.Printf("nitrogen: skip %s (exists, use --force)\n", modelPath)
	}

	if hasInternalLayout() {
		_ = os.MkdirAll("migrations", 0755)
		_ = os.MkdirAll(handlerDir, 0755)
	}

	sql := hasSQLInstalled()
	if sql {
		if err := writeSQLPlatform(); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: platform: %v\n", err)
		}
		_ = ensureLocalNitroReplaces("data/adapter/sql", "data/adapter/pgx", "data/gateway", "blinq")
		applyDBDriverDialect()
		if err := generateBlinqData(modelPath, false); err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: --data: %v\n", err)
		}
		migPath, err := migrationgen.Make(modelPath, migrationgen.MakeOptions{
			Dir:  "migrations",
			Name: "create_" + names.Table,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "nitrogen: migrate:make: %v\n", err)
		} else {
			fmt.Printf("nitrogen: migration created: %s\n", migPath)
		}
		patchRoutesResource(names, fields, true)
		patchMainResource(names, true)
	} else {
		patchRoutesResource(names, fields, false)
		patchMainResource(names, false)
		fmt.Println("nitrogen: tip — nitrogen install postgresql|mysql|sqlite then re-run make:scaffold --resource")
	}
	writeResourceHealthTest(names)
	if !sql {
		writeResourceCRUDTest(names, fields)
	}
	fmt.Println("nitrogen: Resource CRUD — ?name=&active=&sort=-price&page=; ?q= searches the first string field")
}

func patchRoutesResource(names resourceNames, fields []fieldSpec, sql bool) {
	if !hasInternalLayout() {
		return
	}
	_, _, handlerDir, _ := resourceDirs(names.Package)
	path := filepath.Join(handlerDir, "routes.go")
	mod := modulePath()
	cfg := renderResourceConfigLiteral(fields)

	var body string
	if sql {
		body = fmt.Sprintf(`package handler

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"%s/internal/model"
)

// Register mounts health + Resource CRUD for %s (Convention over Configuration).
// store is any typed store (sqladapter.StoreFor[model.%s](db) in main).
func Register(app *nitro.App, store nitro.AsResourceRepository) {
	app.Get("/health", Health)
	app.Resource(&model.%s{}, store, %s)
}
`, mod, names.Type, names.Type, names.Type, cfg)
	} else {
		body = fmt.Sprintf(`package handler

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"%s/internal/model"
)

// Register mounts health + Resource CRUD for %s (in-memory).
func Register(app *nitro.App, repo *nitro.MemoryRepository[model.%s]) {
	app.Get("/health", Health)
	app.Resource(&model.%s{}, repo, %s)
}
`, mod, names.Type, names.Type, names.Type, cfg)
	}

	if b, err := os.ReadFile(path); err == nil {
		src := string(b)
		if strings.Contains(src, "app.Resource(&model."+names.Type) {
			fmt.Printf("nitrogen: routes already register Resource %s\n", names.Type)
			return
		}
		if strings.Contains(src, "func Register(") && (strings.Contains(src, "Repository") || strings.Contains(src, "Store[")) {
			fnName := "Register" + names.Type
			if sql {
				fn := fmt.Sprintf(`

func %s(app *nitro.App, store nitro.AsResourceRepository) {
	app.Resource(&model.%s{}, store, %s)
}
`, fnName, names.Type, cfg)
				src = ensureImport(src, "", mod+"/internal/model")
				_ = writeFileFmt(path, []byte(strings.TrimRight(src, "\n")+fn), 0644)
			} else {
				fn := fmt.Sprintf(`

func %s(app *nitro.App, repo *nitro.MemoryRepository[model.%s]) {
	app.Resource(&model.%s{}, repo, %s)
}
`, fnName, names.Type, names.Type, cfg)
				src = ensureImport(src, "", mod+"/internal/model")
				_ = writeFileFmt(path, []byte(strings.TrimRight(src, "\n")+fn), 0644)
			}
			fmt.Printf("nitrogen: appended %s — call from main\n", fnName)
			return
		}
	}
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s (app.Resource)\n", path)
}

func renderResourceConfigLiteral(fields []fieldSpec) string {
	var filterable []string
	var sortable []string
	for _, f := range fields {
		col := fieldJSONName(f)
		if col == "" {
			continue
		}
		filterable = append(filterable, col)
		sortable = append(sortable, col)
	}
	sortable = append(sortable, "id", "created_at", "updated_at")

	var b strings.Builder
	b.WriteString("&nitro.ResourceConfig{\n")
	b.WriteString("\t\tNoCache: true,\n")
	if len(filterable) > 0 {
		b.WriteString("\t\tFilterable: []string{")
		for i, c := range filterable {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q", c)
		}
		b.WriteString("},\n")
	}
	if len(sortable) > 0 {
		b.WriteString("\t\tSortable: []string{")
		for i, c := range sortable {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q", c)
		}
		b.WriteString("},\n")
	}
	if q := firstStringField(fields); q != nil {
		col := fieldJSONName(*q)
		b.WriteString("\t\tFilterRules: []nitro.FilterRule{\n")
		fmt.Fprintf(&b, "\t\t\t{Param: \"q\", Field: %q, Op: \"icontains\"},\n", col)
		b.WriteString("\t\t},\n")
	}
	b.WriteString("\t}")
	return b.String()
}

func fieldJSONName(f fieldSpec) string {
	tag := f.Tag
	if i := strings.Index(tag, `json:"`); i >= 0 {
		rest := tag[i+6:]
		if j := strings.Index(rest, `"`); j >= 0 {
			n := rest[:j]
			if n != "" && n != "-" {
				return strings.Split(n, ",")[0]
			}
		}
	}
	return strings.ToLower(f.Name)
}

func patchMainResource(names resourceNames, sql bool) {
	path := "cmd/server/main.go"
	if !hasInternalLayout() {
		path = "main.go"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	src := string(b)
	if strings.Contains(src, "app.Resource(&model."+names.Type) ||
		((strings.Contains(src, "StoreOf[model."+names.Type+"]") || strings.Contains(src, "StoreFor[model."+names.Type+"]")) && strings.Contains(src, "handler.Register(app, store)")) {
		fmt.Println("nitrogen: main already wires this Resource")
		return
	}
	mod := modulePath()

	// A previous scaffold already wired handler.Register(app, …): the new
	// resource was appended as Register<Type>, so wire that call too.
	if hasInternalLayout() && strings.Contains(src, "handler.Register(app, ") &&
		!strings.Contains(src, "handler.Register"+names.Type+"(") {
		var wire string
		if sql {
			wire = fmt.Sprintf("	handler.Register%s(app, sqladapter.StoreFor[model.%s](db))\n", names.Type, names.Type)
		} else {
			wire = fmt.Sprintf("	handler.Register%s(app, nitro.NewMemoryRepo[model.%s]())\n", names.Type, names.Type)
		}
		if out, ok := insertBeforeListen(src, wire); ok {
			if sql {
				out = ensureImport(out, "sqladapter", "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2")
			}
			out = ensureImport(out, "", mod+"/internal/model")
			if err := writeFileFmt(path, []byte(out), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "nitrogen: main: %v\n", err)
				return
			}
			fmt.Printf("nitrogen: wired handler.Register%s in main\n", names.Type)
			return
		}
	}

	if sql {
		if hasInternalLayout() && !strings.Contains(src, "sqladapter.OpenEnv") {
			patchSQLMain()
			b, err = os.ReadFile(path)
			if err != nil {
				return
			}
			src = string(b)
		}
		if !strings.Contains(src, "sqladapter") {
			src = strings.Replace(src, "nitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"",
				"sqladapter \"github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2\"\n\tnitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"", 1)
		}
		if hasInternalLayout() {
			modelImp := fmt.Sprintf("\t\"%s/internal/model\"\n", mod)
			if !strings.Contains(src, "/internal/model\"") {
				src = strings.Replace(src, fmt.Sprintf("\t\"%s/internal/handler\"\n", mod),
					modelImp+"\t\""+mod+"/internal/handler\"\n", 1)
			}
			wire := fmt.Sprintf("\tstore := sqladapter.StoreFor[model.%s](db)\n", names.Type)
			switch {
			case strings.Contains(src, "handler.Register(app)"):
				src = strings.Replace(src, "\thandler.Register(app)", wire+"\thandler.Register(app, store)", 1)
			case strings.Contains(src, "handler.Register(app, repo)"):
				src = strings.Replace(src, "handler.Register(app, repo)", "handler.Register(app, store)", 1)
				if !strings.Contains(src, "StoreOf[model."+names.Type) && !strings.Contains(src, "StoreFor[model."+names.Type) {
					src = strings.Replace(src, "handler.Register(app, store)", wire+"\thandler.Register(app, store)", 1)
				}
			default:
				fmt.Printf("nitrogen: tip — wire sqladapter.StoreFor[model.%s](db) + handler.Register(app, store)\n", names.Type)
			}
		} else {
			res := fmt.Sprintf("\tstore := sqladapter.StoreFor[%s](db)\n\tapp.Resource(&%s{}, store, &nitro.ResourceConfig{NoCache: true})\n",
				names.Type, names.Type)
			if strings.Contains(src, "app.Listen") {
				src = strings.Replace(src, "\tapp.Listen", res+"\tapp.Listen", 1)
			}
		}
	} else if hasInternalLayout() {
		modelImp := fmt.Sprintf("\t\"%s/internal/model\"\n", mod)
		if !strings.Contains(src, "/internal/model\"") {
			src = strings.Replace(src, fmt.Sprintf("\t\"%s/internal/handler\"\n", mod),
				modelImp+"\t\""+mod+"/internal/handler\"\n", 1)
		}
		wire := fmt.Sprintf("\trepo := nitro.NewMemoryRepo[model.%s]()\n\thandler.Register(app, repo)", names.Type)
		if strings.Contains(src, "handler.Register(app)") {
			src = strings.Replace(src, "handler.Register(app)", wire, 1)
		}
	}
	if err := writeFileFmt(path, []byte(src), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: main: %v\n", err)
		return
	}
	fmt.Println("nitrogen: wired main with app.Resource")
}

func writeResourceHealthTest(names resourceNames) {
	if !hasInternalLayout() {
		return
	}
	_, _, handlerDir, _ := resourceDirs(names.Package)
	path := filepath.Join(handlerDir, "health_resource_test.go")
	if _, err := os.Stat(path); err == nil {
		return
	}
	body := `package handler_test

import (
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"` + modulePath() + `/internal/handler"
)

func TestHealth(t *testing.T) {
	app := nitro.New()
	app.Get("/health", handler.Health)
	tc := nitro.NewTestClient(app)
	resp := tc.Get(t, "/health")
	tc.AssertStatus(t, resp, 200)
}
`
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
	if hasSQLInstalled() {
		writeStoreOpenTest(handlerDir, names)
	}
}

// writeResourceCRUDTest emits <type>_resource_test.go: create → list → get
// through the generated Register function, so a broken wiring fails `go test`.
func writeResourceCRUDTest(names resourceNames, fields []fieldSpec) {
	if !hasInternalLayout() {
		return
	}
	_, _, handlerDir, _ := resourceDirs(names.Package)
	path := filepath.Join(handlerDir, names.File+"_resource_test.go")
	if _, err := os.Stat(path); err == nil {
		return
	}
	fn := "Register"
	if b, err := os.ReadFile(filepath.Join(handlerDir, "routes.go")); err == nil &&
		strings.Contains(string(b), "func Register"+names.Type+"(") {
		fn = "Register" + names.Type
	}
	var payload []string
	tenant := false
	for _, f := range fields {
		if f.Name == "TenantID" {
			tenant = true
			continue
		}
		var v string
		switch {
		case strings.HasPrefix(f.Type, "string"):
			v = `"sample"`
		case f.Type == "bool":
			v = "true"
		default:
			v = "1"
		}
		payload = append(payload, fmt.Sprintf("%q:%s", fieldJSONName(f), v))
	}
	tenantSetup, tenantClient := "", "tc"
	if tenant {
		// tenant_id column ⇒ the Resource is tenant-scoped by convention. In
		// production the tenant comes from the JWT claim; the test opts in to
		// the header (never do that for untrusted clients).
		tenantSetup = "\tallowHeader := true\n\tapp.Use(nitro.TenantMiddleware(nitro.TenantConfig{Require: true, AllowHeader: &allowHeader}))\n"
		tenantClient = `tc.WithHeader("X-Tenant-ID", "01900000-0000-7000-8000-000000000001")`
	}
	body := fmt.Sprintf(`package handler_test

import (
	"strings"
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"%[1]s/internal/handler"
	"%[1]s/internal/model"
)

func Test%[2]sResourceCRUD(t *testing.T) {
	app := nitro.New(nitro.Config{DisableDefaults: true, Env: nitro.EnvDevelopment})
%[6]s	handler.%[3]s(app, nitro.NewMemoryRepo[model.%[2]s]())
	if err := app.Err(); err != nil {
		t.Fatal(err)
	}
	tc := nitro.NewTestClient(app)
	c := %[7]s

	created := c.Post(t, "/%[4]s", %[5]q)
	tc.AssertStatus(t, created, 201)

	list := c.Get(t, "/%[4]s")
	tc.AssertStatus(t, list, 200)
	if !strings.Contains(string(list.Body()), "sample") && !strings.Contains(string(list.Body()), "\"id\"") {
		t.Fatalf("list must contain the created row: %%s", list.Body())
	}
}
`, modulePath(), names.Type, fn, resourcePathForNames(names), "{"+strings.Join(payload, ",")+"}", tenantSetup, tenantClient)
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
}

func resourcePathForNames(names resourceNames) string { return names.Table }
