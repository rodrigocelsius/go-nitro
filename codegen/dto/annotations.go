package dto

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

const publicStructPrefix = "//nitro:public"

// parsePublicStructs maps struct names that have //nitro:public above the type
// (all exported json fields become public unless nitro:"-").
func parsePublicStructs(filename string) (map[string]bool, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		marked := commentHasPublic(gen.Doc)
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, isStruct := ts.Type.(*ast.StructType); !isStruct {
				continue
			}
			if marked || commentHasPublic(ts.Doc) {
				out[ts.Name.Name] = true
			}
		}
	}
	return out, nil
}

func commentHasPublic(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, c := range doc.List {
		line := strings.TrimSpace(c.Text)
		if strings.HasPrefix(line, publicStructPrefix) || line == publicStructPrefix {
			return true
		}
		// //nitro:public or // nitro:public
		if strings.Contains(line, "nitro:public") && !strings.Contains(line, "nitro:public=") {
			return true
		}
	}
	return false
}
