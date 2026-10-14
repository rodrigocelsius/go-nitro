package nitro

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// This file is the executable CoC contract: every convention the framework
// promises must keep working with ZERO configuration. If a change breaks one
// of these tests, it is a regression of the developer experience, not a test
// to "update" — decide deliberately and document it in the CHANGELOG.

type Person struct {
	Model
	Name string `json:"name"`
}
type Child struct {
	Model
	Name string `json:"name"`
}
type Category struct {
	Model
	Name string `json:"name"`
}
type cocWidget struct {
	Model
	DisplayName string
	UnitPrice   int
}
type cocCustomTable struct {
	Model
	Name string `json:"name"`
}

func (cocCustomTable) TableName() string { return "legacy_things" }

func TestCoC_ResourcePathConvention(t *testing.T) {
	cases := map[string]any{
		"people":        &Person{},
		"children":      &Child{},
		"categories":    &Category{},
		"coc_widgets":   &cocWidget{},
		"legacy_things": &cocCustomTable{}, // TableName() wins
	}
	for want, model := range cases {
		if got := ResourcePath(model); got != want {
			t.Errorf("ResourcePath(%T) = %q, want %q", model, got, want)
		}
	}
}

func TestCoC_DefaultsAreSecureAndObservable(t *testing.T) {
	app := New(Config{DisableStartupMessage: true})
	app.Get("/x", func(c *Ctx) error { return c.OK(Map{"ok": true}) })
	resp, err := app.Test("GET", "/x", nil)
	if err != nil || resp.StatusCode() != 200 {
		t.Fatalf("%v %d", err, resp.StatusCode())
	}
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Referrer-Policy"} {
		if len(resp.Header.Peek(h)) == 0 {
			t.Errorf("security header %s must be on by default", h)
		}
	}
	rid := string(resp.Header.Peek("X-Request-ID"))
	if ok, _ := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, rid); !ok {
		t.Errorf("X-Request-ID must be a UUIDv7 by default, got %q", rid)
	}
}

func TestCoC_ErrorShapeDefaultsToSimpleJSON(t *testing.T) {
	app := New(Config{DisableStartupMessage: true})
	app.Get("/missing", func(c *Ctx) error { return NotFound("nope") })
	resp, _ := app.Test("GET", "/missing", nil)
	if resp.StatusCode() != 404 {
		t.Fatalf("status %d", resp.StatusCode())
	}
	var m map[string]any
	if err := json.Unmarshal(resp.Body(), &m); err != nil || m["error"] != "nope" {
		t.Fatalf("default error body must be {\"error\": msg}, got %s", resp.Body())
	}
	if ct := string(resp.Header.Peek("Content-Type")); strings.Contains(ct, "problem") {
		t.Fatalf("Problem Details must be opt-in, got %q", ct)
	}

	pd := New(Config{DisableStartupMessage: true, ErrorHandler: ProblemDetails()})
	pd.Get("/missing", func(c *Ctx) error { return NotFound("nope") })
	resp, _ = pd.Test("GET", "/missing", nil)
	if !strings.HasPrefix(string(resp.Header.Peek("Content-Type")), "application/problem+json") {
		t.Fatalf("ProblemDetails handler must emit problem+json, got %q", resp.Header.Peek("Content-Type"))
	}
}

func TestCoC_ResourceCRUDContract(t *testing.T) {
	app := New(Config{DisableDefaults: true})
	app.Resource(&Person{}, AdaptRepo(NewMemoryRepo[Person]()), &ResourceConfig{Public: true})

	if err := app.Err(); err != nil {
		t.Fatal(err)
	}
	resp, _ := app.Test("POST", "/people", strings.NewReader(`{"name":"Ana"}`))
	if resp.StatusCode() != 201 {
		t.Fatalf("create → 201, got %d %s", resp.StatusCode(), resp.Body())
	}
	var created map[string]any
	_ = json.Unmarshal(resp.Body(), &created)
	for _, k := range []string{"id", "created_at", "updated_at"} {
		if _, ok := created[k]; !ok {
			t.Errorf("created entity must expose snake_case %q, got %s", k, resp.Body())
		}
	}
	id, _ := created["id"].(string)

	resp, _ = app.Test("GET", "/people", nil)
	var list map[string]any
	_ = json.Unmarshal(resp.Body(), &list)
	for _, k := range []string{"items", "total", "page", "per_page", "total_pages"} {
		if _, ok := list[k]; !ok {
			t.Errorf("list envelope must contain %q, got %s", k, resp.Body())
		}
	}
	resp, _ = app.Test("GET", "/people/"+id, nil)
	if resp.StatusCode() != 200 {
		t.Fatalf("get → 200, got %d", resp.StatusCode())
	}
	resp, _ = app.Test("DELETE", "/people/"+id, nil)
	if resp.StatusCode() != 204 {
		t.Fatalf("delete → 204, got %d", resp.StatusCode())
	}
	resp, _ = app.Test("GET", "/people/"+id, nil)
	if resp.StatusCode() != 404 {
		t.Fatalf("get after delete → 404, got %d", resp.StatusCode())
	}
}

func TestCoC_UntaggedFieldsAreSnakeCaseOnTheWire(t *testing.T) {
	app := New(Config{DisableDefaults: true})
	app.Resource(&cocWidget{}, AdaptRepo(NewMemoryRepo[cocWidget]()), &ResourceConfig{Public: true})
	resp, _ := app.Test("POST", "/coc_widgets", strings.NewReader(`{"display_name":"A","unit_price":5}`))
	if resp.StatusCode() != 201 {
		t.Fatalf("%d %s", resp.StatusCode(), resp.Body())
	}
	if !strings.Contains(string(resp.Body()), `"display_name":"A"`) || !strings.Contains(string(resp.Body()), `"unit_price":5`) {
		t.Fatalf("untagged fields must round-trip as snake_case, got %s", resp.Body())
	}
}

// API-surface guard: names removed in the v2.0.0 cleanup must not come back as
// duplicates of better APIs.
func TestCoC_NoDeprecatedAliasesReintroduced(t *testing.T) {
	banned := map[string]string{
		"OK": "c.OK", "Created": "c.Created", "NoContent": "c.NoContent",
		"CORS": "CORSMiddleware / app.CORS", "Compress": "CompressMiddleware / app.Compress",
		"Timeout": "TimeoutMiddleware / app.Timeout", "MapAny": "Map",
		"DigitsOnly": "Digits", "RegisterValidatorAlias": "RegisterAlias",
		"ResourceOpts": "ResourceConfig", "DefaultResourceOpts": "ResourceConfig{}",
		"WithCache": "ResourceConfig.CacheTTL", "WithoutCache": "ResourceConfig.NoCache",
	}
	fset := token.NewFileSet()
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range af.Decls {
			switch dd := d.(type) {
			case *ast.FuncDecl:
				if dd.Recv == nil {
					if use, bad := banned[dd.Name.Name]; bad {
						t.Errorf("%s: func %s was removed; use %s", f, dd.Name.Name, use)
					}
				}
			case *ast.GenDecl:
				for _, sp := range dd.Specs {
					if ts, ok := sp.(*ast.TypeSpec); ok {
						if use, bad := banned[ts.Name.Name]; bad {
							t.Errorf("%s: type %s was removed; use %s", f, ts.Name.Name, use)
						}
					}
				}
			}
		}
	}
}

// Configuration mistakes must be reported through App.Err (fail at Listen),
// not by panicking in the middle of a fluent chain. Constructors that return
// plain values (no App) keep Must-style panics and are listed explicitly.
func TestCoC_NoNewConfigPanicsInFacade(t *testing.T) {
	allowed := map[string]bool{
		"resource_business.go": true, // Cmd/Q/Rules/When constructors (like regexp.MustCompile)
		"stdhttp.go":           true, // Adapt(nil)
		"singleflight.go":      true, // re-panic of the recovered value
		"ids.go":               true, // Must* helper
		"config.go":            true, // Must* helper
	}
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || allowed[f] {
			continue
		}
		b, _ := os.ReadFile(f)
		if strings.Contains(string(b), "panic(") {
			t.Errorf("%s uses panic(): record the problem with a.fail(err) instead (see App.Err)", f)
		}
	}
}
