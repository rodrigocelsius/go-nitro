package datagen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

const (
	notimestampsPrefix = "//nitro:notimestamps"
	nosoftdeletePrefix = "//nitro:nosoftdelete"
	noidPrefix         = "//nitro:noid"
)

// StructAnnotations stores comment annotations for a struct.
type StructAnnotations struct {
	NoTimestamps bool
	NoSoftDelete bool
	NoID         bool
}

// scanStructAnnotations scans a Go file for //nitro:notimestamps,
// //nitro:nosoftdelete, //nitro:noid comments above struct
// declarations, returning a struct-name -> annotations map.
func scanStructAnnotations(filename string) (map[string]StructAnnotations, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	annotations := make(map[string]StructAnnotations)

	ast.Inspect(file, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE || genDecl.Doc == nil {
			return true
		}
		a := extractStructAnnotations(genDecl.Doc)
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
				annotations[typeSpec.Name.Name] = a
			}
		}
		return true
	})

	return annotations, nil
}

func extractStructAnnotations(doc *ast.CommentGroup) StructAnnotations {
	var a StructAnnotations
	for _, c := range doc.List {
		text := strings.TrimSpace(c.Text)
		switch text {
		case notimestampsPrefix:
			a.NoTimestamps = true
		case nosoftdeletePrefix:
			a.NoSoftDelete = true
		case noidPrefix:
			a.NoID = true
		}
	}
	return a
}

// injectWellKnownFields injects the default fields (ID, CreatedAt,
// UpdatedAt, DeletedAt) that are missing from the user struct. Order is
// preserved: ID first, then user fields, then timestamps.
func injectWellKnownFields(columns []Column, existing map[string]bool, annotations StructAnnotations) []Column {
	var prefix, suffix []Column

	for _, wk := range wellKnownFields {
		if existing[wk.GoName] {
			continue
		}
		switch {
		case wk.GoName == "ID" && annotations.NoID:
			continue
		case (wk.GoName == "CreatedAt" || wk.GoName == "UpdatedAt") && annotations.NoTimestamps:
			continue
		case wk.GoName == "DeletedAt" && annotations.NoSoftDelete:
			continue
		}
		col := Column{
			GoName:     wk.GoName,
			ColumnName: wk.ColumnName,
			Kind:       wk.Kind,
			TypeName:   wk.TypeName,
		}
		if wk.GoName == "ID" {
			prefix = append(prefix, col)
		} else {
			suffix = append(suffix, col)
		}
	}

	result := make([]Column, 0, len(prefix)+len(columns)+len(suffix))
	result = append(result, prefix...)
	result = append(result, columns...)
	result = append(result, suffix...)
	return result
}
