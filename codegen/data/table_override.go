package datagen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

const tableOverridePrefix = "//nitro:table="

// scanTableOverrides scans a Go file for //nitro:table=name comments
// above struct declarations, returning a struct-name -> custom-table-name
// map. Uses go/ast directly (not the shared nitro/gen/ast package)
// because StructDef does not carry the struct's documentation comment —
// that is a datagen-specific need (table-name override).
//
// Usage:
//
//	//nitro:table=app_users
//	type User struct { ... }
func scanTableOverrides(filename string) (map[string]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	overrides := make(map[string]string)

	ast.Inspect(file, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE || genDecl.Doc == nil {
			return true
		}
		override, found := extractTableOverride(genDecl.Doc)
		if !found {
			return true
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
				overrides[typeSpec.Name.Name] = override
			}
		}
		return true
	})

	return overrides, nil
}

func extractTableOverride(doc *ast.CommentGroup) (string, bool) {
	for _, c := range doc.List {
		if strings.HasPrefix(c.Text, tableOverridePrefix) {
			return strings.TrimPrefix(c.Text, tableOverridePrefix), true
		}
	}
	return "", false
}
