package main

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// writeFileFmt is os.WriteFile that gofmt-formats generated Go sources, so
// every scaffold/install/make command emits idiomatic, diff-friendly code.
// Non-Go files and sources that do not parse are written unchanged.
func writeFileFmt(path string, data []byte, perm os.FileMode) error {
	if filepath.Ext(path) == ".go" {
		if out, err := format.Source(data); err == nil {
			data = out
		}
	}
	return os.WriteFile(path, data, perm)
}

// ensureImport adds `alias "path"` to a Go source unless the path is already
// imported. It handles every import shape (none, single line, block), so
// repeated scaffolds keep compiling. Sources that do not parse are returned
// unchanged.
func ensureImport(src, alias, path string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ImportsOnly)
	if err != nil {
		return src
	}
	for _, im := range f.Imports {
		if p, _ := strconv.Unquote(im.Path.Value); p == path {
			return src
		}
	}
	spec := strconv.Quote(path)
	if alias != "" {
		spec = alias + " " + spec
	}
	var last *ast.GenDecl
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			last = g
		}
	}
	switch {
	case last == nil:
		off := fset.Position(f.Name.End()).Offset
		return src[:off] + "\n\nimport " + spec + "\n" + src[off:]
	case last.Lparen.IsValid():
		off := fset.Position(last.Rparen).Offset
		return src[:off] + "\t" + spec + "\n" + src[off:]
	default:
		start := fset.Position(last.Pos()).Offset
		end := fset.Position(last.End()).Offset
		existing := strings.TrimSpace(strings.TrimPrefix(src[start:end], "import"))
		return src[:start] + "import (\n\t" + existing + "\n\n\t" + spec + "\n)" + src[end:]
	}
}
