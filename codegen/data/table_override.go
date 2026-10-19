package datagen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
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

	// `func (T) TableName() string { return "x" }` is what the runtime
	// (Resource, StoreFor) honors; the metamodel must agree. An explicit
	// //nitro:table= comment still wins.
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "TableName" || fn.Recv == nil || len(fn.Recv.List) != 1 || fn.Body == nil {
			continue
		}
		recv := fn.Recv.List[0].Type
		if star, ok := recv.(*ast.StarExpr); ok {
			recv = star.X
		}
		ident, ok := recv.(*ast.Ident)
		if !ok {
			continue
		}
		if _, exists := overrides[ident.Name]; exists {
			continue
		}
		if name, ok := literalReturn(fn.Body); ok {
			overrides[ident.Name] = name
		}
	}

	return overrides, nil
}

// literalReturn extracts "x" from a body that is exactly `return "x"`.
func literalReturn(body *ast.BlockStmt) (string, bool) {
	if len(body.List) != 1 {
		return "", false
	}
	ret, ok := body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return "", false
	}
	lit, ok := ret.Results[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil || v == "" {
		return "", false
	}
	return v, true
}

func extractTableOverride(doc *ast.CommentGroup) (string, bool) {
	for _, c := range doc.List {
		if strings.HasPrefix(c.Text, tableOverridePrefix) {
			return strings.TrimPrefix(c.Text, tableOverridePrefix), true
		}
	}
	return "", false
}
