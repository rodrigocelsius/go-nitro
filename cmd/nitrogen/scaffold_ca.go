package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	datagen "github.com/rodrigocelsius/go-nitro/codegen/data/v2"
	migrationgen "github.com/rodrigocelsius/go-nitro/codegen/migration/v2"
)

func hasSQLInstalled() bool {
	for _, p := range []string{"internal/config/config.go", "config/config.go"} {
		b, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(b), "type DBConfig struct") {
			return true
		}
	}
	if os.Getenv("NITRO_DB_DRIVER") != "" {
		return true
	}
	return dotenvValue("NITRO_DB_DRIVER") != ""
}

func applyDBDriverDialect() {
	d := os.Getenv("NITRO_DB_DRIVER")
	if d == "" {
		d = dotenvValue("NITRO_DB_DRIVER")
	}
	if d != "" {
		migrationgen.SetDialect(migrationgen.DialectFromDriver(d))
	}
}

func dotenvValue(key string) string {
	b, err := os.ReadFile(".env")
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func firstStringField(fields []fieldSpec) *fieldSpec {
	for i := range fields {
		if fields[i].Type == "string" {
			return &fields[i]
		}
	}
	return nil
}

func activeField(fields []fieldSpec) *fieldSpec {
	for i := range fields {
		if fields[i].Name == "Active" && fields[i].Type == "bool" {
			return &fields[i]
		}
	}
	return nil
}

func dslAlias(typeName string) string {
	return exportIdent(pluralize(typeName))
}

// renderBlinqOrderBy emits sort from f.Sort (?sort=name|-price) with CreatedAt desc default.
func renderBlinqOrderBy(alias string, fields []fieldSpec) string {
	var b strings.Builder
	b.WriteString("\tswitch strings.TrimPrefix(f.Sort, \"+\") {\n")
	seen := map[string]bool{}
	for _, f := range fields {
		col := fieldJSONName(f)
		if col == "" || seen[col] {
			continue
		}
		seen[col] = true
		fmt.Fprintf(&b, "\tcase %q, %q:\n", col, f.Name)
		fmt.Fprintf(&b, "\t\tif strings.HasPrefix(f.Sort, \"-\") {\n")
		fmt.Fprintf(&b, "\t\t\tq = q.OrderBy(modelgen.%s.%s.OrderDesc())\n", alias, f.Name)
		fmt.Fprintf(&b, "\t\t} else {\n")
		fmt.Fprintf(&b, "\t\t\tq = q.OrderBy(modelgen.%s.%s.OrderAsc())\n", alias, f.Name)
		fmt.Fprintf(&b, "\t\t}\n")
		fmt.Fprintf(&b, "\tcase \"-%s\":\n", col)
		fmt.Fprintf(&b, "\t\tq = q.OrderBy(modelgen.%s.%s.OrderDesc())\n", alias, f.Name)
	}
	fmt.Fprintf(&b, "\tdefault:\n\t\tq = q.OrderBy(modelgen.%s.CreatedAt.OrderDesc())\n", alias)
	b.WriteString("\t}\n")
	return b.String()
}

func renderModelFilePkg(pkg, name, table string, fields []fieldSpec) string {
	needTime := false
	for _, f := range fields {
		if f.Type == "time.Time" {
			needTime = true
			break
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	b.WriteString("import (\n")
	if needTime {
		b.WriteString("\t\"time\"\n\n")
	}
	b.WriteString("\tnitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"\n")
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "//go:generate nitrogen make %s.go --data\n\n", strings.ToLower(name))
	fmt.Fprintf(&b, "//nitro:table=%s\n", table)
	fmt.Fprintf(&b, "type %s struct {\n", name)
	b.WriteString("\tnitro.Model\n")
	for _, f := range fields {
		fmt.Fprintf(&b, "\t%s %s %s\n", f.Name, f.Type, f.Tag)
	}
	b.WriteString("}\n")
	if table != datagen.TableNameFromStruct(name) {
		fmt.Fprintf(&b, "\nfunc (%s) TableName() string { return %q }\n", name, table)
	}
	return b.String()
}

func renderUsecaseFile(name, op string) string {
	pkg := strings.ToLower(name)
	mod := modulePath()
	if !hasInternalLayout() {
		switch op {
		case "create":
			return fmt.Sprintf("package %s\n\nimport \"context\"\n\nfunc Create(ctx context.Context, repo Repository, in *%s) (*%s, error) {\n\treturn repo.Create(ctx, in)\n}\n", pkg, name, name)
		case "list":
			return fmt.Sprintf("package %s\n\nimport \"context\"\n\nfunc List(ctx context.Context, repo Repository, f Filter) ([]%s, error) {\n\treturn repo.FindAll(ctx, f)\n}\n", pkg, name)
		}
	}
	switch op {
	case "create":
		return fmt.Sprintf("package %s\n\nimport (\n\t\"context\"\n\n\t\"%s/internal/model\"\n)\n\nfunc Create(ctx context.Context, repo Repository, in *model.%s) (*model.%s, error) {\n\treturn repo.Create(ctx, in)\n}\n", pkg, mod, name, name)
	case "get":
		return fmt.Sprintf("package %s\n\nimport (\n\t\"context\"\n\n\t\"%s/internal/model\"\n)\n\nfunc Get(ctx context.Context, repo Repository, id string) (*model.%s, error) {\n\treturn repo.FindByID(ctx, id)\n}\n", pkg, mod, name)
	case "list":
		return fmt.Sprintf("package %s\n\nimport (\n\t\"context\"\n\n\t\"%s/internal/model\"\n)\n\nfunc List(ctx context.Context, repo Repository, f Filter) ([]model.%s, error) {\n\treturn repo.FindAll(ctx, f)\n}\n", pkg, mod, name)
	case "update":
		return fmt.Sprintf("package %s\n\nimport (\n\t\"context\"\n\n\t\"%s/internal/model\"\n)\n\nfunc Update(ctx context.Context, repo Repository, in *model.%s) (*model.%s, error) {\n\tif err := repo.Update(ctx, in); err != nil {\n\t\treturn nil, err\n\t}\n\treturn in, nil\n}\n", pkg, mod, name, name)
	case "delete":
		return fmt.Sprintf("package %s\n\nimport \"context\"\n\nfunc Delete(ctx context.Context, repo Repository, id string) error {\n\treturn repo.Delete(ctx, id)\n}\n", pkg)
	}
	return ""
}

func renderCustomUsecaseFile(resourceType, opType string) string {
	pkg := strings.ToLower(resourceType)
	mod := modulePath()
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	b.WriteString("import (\n\t\"context\"\n")
	if hasInternalLayout() && mod != "" {
		fmt.Fprintf(&b, "\n\t\"%s/internal/model\"\n", mod)
	}
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "// %s implements a domain use case for %s.\n", opType, resourceType)
	if hasInternalLayout() && mod != "" {
		fmt.Fprintf(&b, "func %s(ctx context.Context, repo Repository, in *model.%s) (*model.%s, error) {\n", opType, resourceType, resourceType)
		b.WriteString("\t_ = ctx\n\t_ = repo\n\t_ = in\n\treturn in, nil\n}\n")
	} else {
		fmt.Fprintf(&b, "func %s(ctx context.Context, repo Repository, in *%s) (*%s, error) {\n", opType, resourceType, resourceType)
		b.WriteString("\t_ = ctx\n\t_ = repo\n\t_ = in\n\treturn in, nil\n}\n")
	}
	return b.String()
}

func renderHandlerFile(name, table string, fields []fieldSpec) string {
	if !hasInternalLayout() {
		return fmt.Sprintf("package main\n\nimport nitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"\n\nfunc List%s(c *nitro.Ctx) error { return c.OK(nitro.Map{\"items\": []any{}}) }\n", name)
	}
	mod := modulePath()
	pkgUC := strings.ToLower(name)
	var b strings.Builder
	fmt.Fprintf(&b, "package handler\n\n")
	b.WriteString("import (\n")
	b.WriteString("\tnitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"\n\n")
	fmt.Fprintf(&b, "\t\"%s/internal/model\"\n", mod)
	fmt.Fprintf(&b, "\t%suc \"%s/internal/usecase/%s\"\n", pkgUC, mod, pkgUC)
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "type %sHandler struct {\n\trepo %suc.Repository\n}\n\n", name, pkgUC)
	fmt.Fprintf(&b, "func (h *%sHandler) List(c *nitro.Ctx) error {\n", name)
	fmt.Fprintf(&b, "\tpage := c.Int(\"page\", 1)\n\tper := c.Int(\"per_page\", 20)\n")
	fmt.Fprintf(&b, "\titems, err := %suc.List(c, h.repo, %suc.Filter{\n", pkgUC, pkgUC)
	if firstStringField(fields) != nil {
		b.WriteString("\t\tQ:      c.Query(\"q\"),\n")
	}
	if activeField(fields) != nil {
		b.WriteString("\t\tActive: c.Query(\"active\"),\n")
	}
	b.WriteString("\t\tSort:    c.Query(\"sort\"),\n")
	b.WriteString("\t\tPage:    page,\n\t\tPerPage: per,\n")
	b.WriteString("\t})\n\tif err != nil {\n\t\treturn err\n\t}\n")
	b.WriteString("\treturn c.OK(nitro.Map{\"items\": items, \"total\": len(items), \"page\": page, \"per_page\": per})\n}\n\n")
	fmt.Fprintf(&b, "func (h *%sHandler) Get(c *nitro.Ctx) error {\n\tp, err := %suc.Get(c, h.repo, c.Param(\"id\"))\n\tif err != nil {\n\t\treturn err\n\t}\n\treturn c.OK(p)\n}\n\n", name, pkgUC)
	fmt.Fprintf(&b, "func (h *%sHandler) Create(c *nitro.Ctx) error {\n\tin, err := nitro.Parse[model.%s](c)\n\tif err != nil {\n\t\treturn err\n\t}\n\tp, err := %suc.Create(c, h.repo, &in)\n\tif err != nil {\n\t\treturn err\n\t}\n\treturn c.Created(p)\n}\n\n", name, name, pkgUC)
	fmt.Fprintf(&b, "func (h *%sHandler) Update(c *nitro.Ctx) error {\n\tin, err := nitro.Parse[model.%s](c)\n\tif err != nil {\n\t\treturn err\n\t}\n\tin.ID, err = nitro.ParseUUID(c.Param(\"id\"))\n\tif err != nil {\n\t\treturn nitro.BadRequest(\"invalid id\")\n\t}\n\tp, err := %suc.Update(c, h.repo, &in)\n\tif err != nil {\n\t\treturn err\n\t}\n\treturn c.OK(p)\n}\n\n", name, name, pkgUC)
	fmt.Fprintf(&b, "func (h *%sHandler) Delete(c *nitro.Ctx) error {\n\tif err := %suc.Delete(c, h.repo, c.Param(\"id\")); err != nil {\n\t\treturn err\n\t}\n\treturn c.NoContent()\n}\n", name, pkgUC)
	_ = table
	return b.String()
}

func renderRepoMemory(name string, fields []fieldSpec) string {
	pkg := "repository"
	if !hasInternalLayout() {
		return fmt.Sprintf("package main\n\ntype %sRepository struct{}\n", name)
	}
	mod := modulePath()
	uc := strings.ToLower(name)
	q := firstStringField(fields)
	act := activeField(fields)
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	b.WriteString("import (\n\t\"context\"\n")
	if q != nil {
		b.WriteString("\t\"strings\"\n")
	}
	b.WriteString("\n\tnitro \"github.com/rodrigocelsius/go-nitro/pkg/v2\"\n")
	fmt.Fprintf(&b, "\t\"%s/internal/model\"\n", mod)
	fmt.Fprintf(&b, "\t%suc \"%s/internal/usecase/%s\"\n", uc, mod, uc)
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "type %sMemory struct {\n\tinner *nitro.MemoryRepository[model.%s]\n}\n\n", name, name)
	fmt.Fprintf(&b, "func New%sMemory() *%sMemory {\n\treturn &%sMemory{inner: nitro.NewMemoryRepo[model.%s]()}\n}\n\n", name, name, name, name)
	fmt.Fprintf(&b, "var _ %suc.Repository = (*%sMemory)(nil)\n\n", uc, name)
	fmt.Fprintf(&b, "func (r *%sMemory) Create(ctx context.Context, in *model.%s) (*model.%s, error) {\n\tif err := r.inner.Create(ctx, in); err != nil {\n\t\treturn nil, err\n\t}\n\treturn in, nil\n}\n\n", name, name, name)
	fmt.Fprintf(&b, "func (r *%sMemory) FindByID(ctx context.Context, id string) (*model.%s, error) {\n\treturn r.inner.FindByID(ctx, id)\n}\n\n", name, name)
	fmt.Fprintf(&b, "func (r *%sMemory) FindAll(ctx context.Context, f %suc.Filter) ([]model.%s, error) {\n", name, uc, name)
	b.WriteString("\tres, err := r.inner.List(ctx, 1, 10000)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n")
	fmt.Fprintf(&b, "\tout := make([]model.%s, 0, len(res.Items))\n", name)
	if q != nil {
		b.WriteString("\tq := strings.ToLower(f.Q)\n")
	}
	fmt.Fprintf(&b, "\tfor _, it := range res.Items {\n\t\tp, ok := it.(*model.%s)\n\t\tif !ok {\n\t\t\tcontinue\n\t\t}\n", name)
	if act != nil {
		b.WriteString("\t\tif f.Active == \"true\" && !p.Active {\n\t\t\tcontinue\n\t\t}\n\t\tif f.Active == \"false\" && p.Active {\n\t\t\tcontinue\n\t\t}\n")
	}
	if q != nil {
		fmt.Fprintf(&b, "\t\tif q != \"\" && !strings.Contains(strings.ToLower(p.%s), q) {\n\t\t\tcontinue\n\t\t}\n", q.Name)
	}
	b.WriteString("\t\tout = append(out, *p)\n\t}\n")
	b.WriteString("\tpage, per := f.Page, f.PerPage\n\tif page < 1 {\n\t\tpage = 1\n\t}\n\tif per < 1 {\n\t\tper = 20\n\t}\n")
	b.WriteString("\tstart := (page - 1) * per\n\tif start > len(out) {\n\t\tstart = len(out)\n\t}\n\tend := start + per\n\tif end > len(out) {\n\t\tend = len(out)\n\t}\n")
	b.WriteString("\treturn out[start:end], nil\n}\n\n")
	fmt.Fprintf(&b, "func (r *%sMemory) Update(ctx context.Context, in *model.%s) error {\n\treturn r.inner.Update(ctx, in)\n}\n\n", name, name)
	fmt.Fprintf(&b, "func (r *%sMemory) Delete(ctx context.Context, id string) error {\n\treturn r.inner.Delete(ctx, id)\n}\n", name)
	return b.String()
}

func renderRepoBlinq(name, table string, fields []fieldSpec) string {
	mod := modulePath()
	uc := strings.ToLower(name)
	alias := dslAlias(name)
	q := firstStringField(fields)
	act := activeField(fields)
	var b strings.Builder
	b.WriteString("package repository\n\n")
	b.WriteString("import (\n\t\"context\"\n\t\"strings\"\n\n")
	b.WriteString("\tsqladapter \"github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2\"\n")
	fmt.Fprintf(&b, "\t\"%s/internal/model\"\n", mod)
	fmt.Fprintf(&b, "\t\"%s/internal/modelgen\"\n", mod)
	fmt.Fprintf(&b, "\t%suc \"%s/internal/usecase/%s\"\n", uc, mod, uc)
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "type %sRepository struct {\n\tstore *sqladapter.Store[%s]\n}\n\n", name, "model."+name)
	fmt.Fprintf(&b, "func New%s(store *sqladapter.Store[model.%s]) *%sRepository {\n\treturn &%sRepository{store: store}\n}\n\n", name, name, name, name)
	fmt.Fprintf(&b, "var _ %suc.Repository = (*%sRepository)(nil)\n\n", uc, name)
	fmt.Fprintf(&b, "func (r *%sRepository) Create(ctx context.Context, in *model.%s) (*model.%s, error) {\n\treturn r.store.CreateOne(ctx, in)\n}\n\n", name, name, name)
	fmt.Fprintf(&b, "func (r *%sRepository) FindByID(ctx context.Context, id string) (*model.%s, error) {\n\treturn r.store.Get(ctx, id)\n}\n\n", name, name)
	fmt.Fprintf(&b, "func (r *%sRepository) FindAll(ctx context.Context, f %suc.Filter) ([]model.%s, error) {\n", name, uc, name)
	b.WriteString("\tpage, per := f.Page, f.PerPage\n\tif page < 1 {\n\t\tpage = 1\n\t}\n\tif per < 1 {\n\t\tper = 20\n\t}\n")
	b.WriteString("\tq := r.store.Query()\n")
	if act != nil {
		fmt.Fprintf(&b, "\tq = q.WhereIf(f.Active == \"true\", modelgen.%s.Active.Eq(true))\n", alias)
		fmt.Fprintf(&b, "\tq = q.WhereIf(f.Active == \"false\", modelgen.%s.Active.Eq(false))\n", alias)
	}
	if q != nil {
		fmt.Fprintf(&b, "\tq = q.WhereIf(f.Q != \"\", modelgen.%s.%s.IContains(f.Q))\n", alias, q.Name)
	}
	b.WriteString(renderBlinqOrderBy(alias, fields))
	b.WriteString("\tq = q.Paginate(uint64(page), uint64(per))\n")
	b.WriteString("\treturn r.store.Find(ctx, q)\n}\n\n")
	fmt.Fprintf(&b, "func (r *%sRepository) Update(ctx context.Context, in *model.%s) error {\n", name, name)
	if len(fields) == 0 {
		b.WriteString("\treturn nil\n}\n\n")
	} else {
		b.WriteString("\tupdated, err := r.store.Update(ctx, in.ID")
		for _, f := range fields {
			fmt.Fprintf(&b, ",\n\t\tmodelgen.%s.%s.Set(in.%s)", alias, f.Name, f.Name)
		}
		b.WriteString(",\n\t)\n\tif err != nil {\n\t\treturn err\n\t}\n\t*in = *updated\n\treturn nil\n}\n\n")
	}
	fmt.Fprintf(&b, "func (r *%sRepository) Delete(ctx context.Context, id string) error {\n\treturn r.store.Delete(ctx, id)\n}\n", name)
	_ = table
	return b.String()
}

func ensureUsecaseRepoInterface(name string, fields []fieldSpec) {
	_, usecaseDir, _, _ := resourceDirs(name)
	if !hasInternalLayout() {
		return
	}
	mod := modulePath()
	path := filepath.Join(usecaseDir, "repository.go")
	q := firstStringField(fields)
	act := activeField(fields)
	var filter strings.Builder
	filter.WriteString("type Filter struct {\n")
	if q != nil {
		filter.WriteString("\tQ string\n")
	}
	if act != nil {
		filter.WriteString("\tActive string\n")
	}
	filter.WriteString("\tSort    string // e.g. name, -price, created_at (default -created_at)\n")
	filter.WriteString("\tPage    int\n\tPerPage int\n}\n")
	body := fmt.Sprintf(`package %s

import (
	"context"

	"%s/internal/model"
)

%s
// Repository is the persistence port for %s usecases.
type Repository interface {
	Create(ctx context.Context, in *model.%s) (*model.%s, error)
	FindByID(ctx context.Context, id string) (*model.%s, error)
	FindAll(ctx context.Context, f Filter) ([]model.%s, error)
	Update(ctx context.Context, in *model.%s) error
	Delete(ctx context.Context, id string) error
}
`, strings.ToLower(name), mod, filter.String(), name, name, name, name, name, name)
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
}

func patchRoutes(names resourceNames) {
	if !hasInternalLayout() {
		return
	}
	_, _, handlerDir, _ := resourceDirs(names.Package)
	path := filepath.Join(handlerDir, "routes.go")
	mod := modulePath()
	uc := strings.ToLower(names.Type)
	table := names.Table
	mount := fmt.Sprintf(`	h := &%sHandler{repo: repo}
	app.Get("/%s", h.List)
	app.Get("/%s/:id", h.Get)
	app.Post("/%s", h.Create)
	app.Patch("/%s/:id", h.Update)
	app.Delete("/%s/:id", h.Delete)
`, names.Type, table, table, table, table, table)

	b, err := os.ReadFile(path)
	if err != nil {
		body := fmt.Sprintf(`package handler

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	%suc "%s/internal/usecase/%s"
)

func Register(app *nitro.App, repo %suc.Repository) {
	app.Get("/health", Health)
%s}
`, uc, mod, uc, uc, mount)
		if err := writeFile(path, body); err != nil {
			fatalWrite(err)
		}
		fmt.Printf("nitrogen: wrote %s\n", path)
		return
	}
	src := string(b)
	if strings.Contains(src, "&"+names.Type+"Handler{") {
		return
	}
	if strings.Contains(src, "func Register(app *nitro.App)") && !strings.Contains(src, "Repository") {
		body := fmt.Sprintf(`package handler

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	%suc "%s/internal/usecase/%s"
)

func Register(app *nitro.App, repo %suc.Repository) {
	app.Get("/health", Health)
%s}
`, uc, mod, uc, uc, mount)
		if err := writeFile(path, body); err != nil {
			fatalWrite(err)
		}
		fmt.Printf("nitrogen: updated %s\n", path)
		return
	}
	fn := fmt.Sprintf(`

func Register%s(app *nitro.App, repo %suc.Repository) {
%s}
`, names.Type, uc, mount)
	src = ensureImport(src, uc+"uc", mod+"/internal/usecase/"+uc)
	_ = writeFileFmt(path, []byte(strings.TrimRight(src, "\n")+fn), 0644)
	fmt.Printf("nitrogen: appended Register%s — call it from Register/main\n", names.Type)
}

func writeHandlerTest(names resourceNames) {
	if !hasInternalLayout() {
		return
	}
	_, _, handlerDir, _ := resourceDirs(names.Package)
	path := filepath.Join(handlerDir, names.File+"_test.go")
	if _, err := os.Stat(path); err == nil {
		return
	}
	mod := modulePath()
	// patchRoutes ran first: a resource after the first one is mounted by
	// Register<Type>, not Register.
	register := "Register"
	if routes, err := os.ReadFile(filepath.Join(handlerDir, "routes.go")); err == nil &&
		strings.Contains(string(routes), "func Register"+names.Type+"(") {
		register += names.Type
	}
	body := fmt.Sprintf(`package handler_test

import (
	"testing"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"%s/internal/handler"
	"%s/internal/repository"
)

func TestList%sEmpty(t *testing.T) {
	app := nitro.New()
	handler.%s(app, repository.New%sMemory())
	tc := nitro.NewTestClient(app)
	resp := tc.Get(t, "/%s")
	tc.AssertStatus(t, resp, 200)
}
`, mod, mod, names.Type, register, names.Type, names.Table)
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)

	if hasSQLInstalled() {
		writeStoreOpenTest(handlerDir, names)
	}
}

func writeStoreOpenTest(handlerDir string, names resourceNames) {
	mod := modulePath()
	if mod == "" {
		return
	}
	path := filepath.Join(handlerDir, names.File+"_store_test.go")
	if _, err := os.Stat(path); err == nil {
		return
	}
	body := fmt.Sprintf(`package handler_test

import (
	"context"
	"path/filepath"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"

	"%s/internal/model"
)

func Test%sStore_OpenTest(t *testing.T) {
	ctx := context.Background()
	// go test ./internal/handler → cwd is this package; migrations live at module root.
	mig := filepath.Join("..", "..", "migrations")
	db, err := sqladapter.OpenTestMigrate(ctx, mig)
	if err != nil {
		t.Fatalf("OpenTestMigrate: %%v (need migrations/ — nitrogen migrate:make)", err)
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
`, mod, names.Type, names.Type, names.Table, names.Type)
	if err := writeFile(path, body); err != nil {
		fatalWrite(err)
	}
	fmt.Printf("nitrogen: wrote %s\n", path)
}

func patchMainWithRepo(names resourceNames) {
	path := "cmd/server/main.go"
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	src := string(b)
	if strings.Contains(src, "repository.New"+names.Type+"(") {
		return
	}
	mod := modulePath()
	if !strings.Contains(src, "sqladapter.OpenEnv") {
		patchSQLMain()
		b, err = os.ReadFile(path)
		if err != nil {
			return
		}
		src = string(b)
	}
	wire := fmt.Sprintf("\tstore := sqladapter.StoreOf[model.%s](db, %q)\n\trepo := repository.New%s(store)\n\n", names.Type, names.Table, names.Type)
	switch {
	case strings.Contains(src, "\thandler.Register(app)"):
		src = strings.Replace(src, "\thandler.Register(app)", wire+"\thandler.Register(app, repo)", 1)
	case strings.Contains(src, "handler.Register(app, ") && !strings.Contains(src, "handler.Register"+names.Type+"("):
		// An earlier scaffold owns handler.Register; this one was appended
		// to routes.go as Register<Type>, so wire that call too.
		call := fmt.Sprintf("\thandler.Register%s(app, repository.New%s(sqladapter.StoreOf[model.%s](db, %q)))\n", names.Type, names.Type, names.Type, names.Table)
		var wired bool
		if src, wired = insertBeforeListen(src, call); !wired {
			fmt.Printf("nitrogen: tip — add %s in %s\n", strings.TrimSpace(call), path)
			return
		}
	default:
		fmt.Printf("nitrogen: tip — wire repository.New%s(sqladapter.StoreOf[model.%s](db, %q)) in %s\n", names.Type, names.Type, names.Table, path)
		return
	}
	src = ensureImport(src, "sqladapter", "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2")
	src = ensureImport(src, "", mod+"/internal/model")
	src = ensureImport(src, "", mod+"/internal/repository")
	if err := writeFileFmt(path, []byte(src), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: main: %v\n", err)
		return
	}
	fmt.Printf("nitrogen: wired cmd/server/main.go with repository.New%s\n", names.Type)
}

// insertBeforeListen inserts code right before the app.Listen call in main.
func insertBeforeListen(src, code string) (string, bool) {
	for _, anchor := range []string{"\tnitro.Ok(app.Listen", "\tapp.Listen", "\tif err := app.Listen"} {
		if strings.Contains(src, anchor) {
			return strings.Replace(src, anchor, code+"\n"+anchor, 1), true
		}
	}
	return src, false
}

func patchMainMemoryRepo(names resourceNames) {
	path := "cmd/server/main.go"
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	src := string(b)
	if strings.Contains(src, "repository.New"+names.Type+"Memory(") {
		return
	}
	if strings.Contains(src, "sqladapter.OpenEnv") {
		return
	}
	switch {
	case strings.Contains(src, "handler.Register(app)"):
		wire := fmt.Sprintf("\trepo := repository.New%sMemory()\n\thandler.Register(app, repo)", names.Type)
		src = strings.Replace(src, "handler.Register(app)", wire, 1)
	case strings.Contains(src, "handler.Register(app, ") && !strings.Contains(src, "handler.Register"+names.Type+"("):
		call := fmt.Sprintf("\thandler.Register%s(app, repository.New%sMemory())\n", names.Type, names.Type)
		var wired bool
		if src, wired = insertBeforeListen(src, call); !wired {
			fmt.Printf("nitrogen: tip — add %s in %s\n", strings.TrimSpace(call), path)
			return
		}
	default:
		return
	}
	src = ensureImport(src, "", modulePath()+"/internal/repository")
	if err := writeFileFmt(path, []byte(src), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "nitrogen: main: %v\n", err)
		return
	}
	fmt.Println("nitrogen: wired cmd/server/main.go with in-memory repository")
}
