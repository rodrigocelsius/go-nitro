package datagen

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	nitroast "github.com/rodrigocelsius/go-nitro/codegen/ast/v2"
)

// embedFlags summarizes what well-known Nitro bases a struct embeds,
// directly or through local base structs.
type embedFlags struct {
	hard   bool // nitro.HardModel: no deleted_at column
	tenant bool // nitro.TenantModel: tenant_id column
}

// packageStructs parses every non-test .go file next to filename and returns
// its structs by name. Embedded base structs commonly live in a sibling file
// (e.g. a shared `type InstanceModel struct{ nitro.HardModel; ... }`), and the
// single-file AST parser cannot see them.
func packageStructs(filename string) map[string]nitroast.StructDef {
	out := make(map[string]nitroast.StructDef)
	dir := filepath.Dir(filename)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_gen.go") {
			continue
		}
		defs, err := nitroast.ParseFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, d := range defs {
			out[d.Name] = d
		}
	}
	return out
}

// embeddedBases returns the names of package structs embedded by another
// package struct (abstract bases such as `type Scoped struct{ nitro.Model }`).
func embeddedBases(pkg map[string]nitroast.StructDef) map[string]bool {
	out := make(map[string]bool)
	for _, d := range pkg {
		for _, emb := range d.Embeds {
			name := strings.TrimPrefix(emb, "*")
			if !strings.Contains(name, ".") {
				if _, ok := pkg[name]; ok {
					out[name] = true
				}
			}
		}
	}
	return out
}

// resolveEmbeds returns the fields promoted from local embedded structs that
// the per-file parser left unexpanded, plus the well-known base flags.
// Fields already present on def win (Go shadowing semantics).
func resolveEmbeds(def nitroast.StructDef, pkg map[string]nitroast.StructDef) ([]nitroast.Field, embedFlags) {
	var flags embedFlags
	seen := make(map[string]bool, len(def.Fields))
	for _, f := range def.Fields {
		seen[f.GoName] = true
	}
	var promoted []nitroast.Field
	visiting := map[string]bool{def.Name: true}

	var walk func(embeds []string)
	walk = func(embeds []string) {
		for _, emb := range embeds {
			base := strings.TrimPrefix(emb, "*")
			if i := strings.LastIndexByte(base, '.'); i >= 0 {
				switch base[i+1:] {
				case "HardModel":
					flags.hard = true
				case "TenantModel":
					flags.tenant = true
				}
				continue
			}
			sd, ok := pkg[base]
			if !ok || visiting[base] {
				continue
			}
			visiting[base] = true
			walk(sd.Embeds)
			for _, f := range sd.Fields {
				if seen[f.GoName] {
					continue
				}
				seen[f.GoName] = true
				promoted = append(promoted, f)
			}
		}
	}
	walk(def.Embeds)
	return promoted, flags
}

// fileImports maps the package identifiers used in filename to import paths.
func fileImports(filename string) map[string]string {
	out := make(map[string]string)
	f, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
	if err != nil {
		return out
	}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		if name == "" {
			name = path[strings.LastIndexByte(path, '/')+1:]
			// Major-version suffix: github.com/x/pkg/v2 → pkg is declared by
			// the package clause; best effort: use the previous element.
			if len(name) > 1 && name[0] == 'v' && strings.Trim(name[1:], "0123456789") == "" {
				trimmed := strings.TrimSuffix(path, "/"+name)
				name = trimmed[strings.LastIndexByte(trimmed, '/')+1:]
			}
		}
		out[name] = path
	}
	return out
}
