// Package ast (nitro/gen/ast) implements the shared AST parser used by
// all Nitro code generators (json/gen, data/gen, and future ones).
// Extracted in Phase 7 from the original Phase 4 parser
// (json/gen/parser.go) so struct/tag/type extraction is not duplicated
// across generators with different purposes.
//
// Unlike the original parser (which only looked at the `json` tag),
// this extracts ALL tags for each field (Tags map[string]string),
// leaving each consumer to pick which tag to use (json, db, etc.) and
// to decide its own field include/exclude rules.
package ast

import (
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// FieldKind categorizes a struct field to decide which code template
// to generate.
type FieldKind int

const (
	KindString FieldKind = iota
	KindInt
	KindFloat
	KindBool
	KindStruct
	KindStructPtr
	KindSlice
	KindMap
	KindTime
	KindUnsupported
)

// Field represents a struct field already resolved for code generation.
type Field struct {
	GoName string
	// Tags holds every field tag, keyed by tag name.
	Tags         map[string]string
	Kind         FieldKind
	TypeName     string
	ElemKind     FieldKind
	ElemTypeName string
}

// StructDef represents a struct extracted from source.
type StructDef struct {
	Name string
	// Fields lists every declared field. Embedded structs declared in this
	// same file are expanded inline (promoted fields included); embeds that
	// cannot be resolved here stay listed in Embeds.
	Fields []Field
	// Embeds lists embedded type names not expanded by the parser
	// (cross-file / cross-package types, e.g. nitro.Model).
	Embeds []string
}

// ParseFile parses a Go file and returns every struct found, with all
// supported fields — filtering by a specific tag is the caller's job.
//
// Multi-name fields (A, B string) yield one Field per name; import aliases
// are resolved against the file's imports (so `t "time"` + t.Time is a Time);
// embedded structs declared in the same file have their promoted fields
// expanded inline.
func ParseFile(filename string) ([]StructDef, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("ast: failed to parse %s: %w", filename, err)
	}
	aliases := importAliases(file)

	var defs []StructDef

	goast.Inspect(file, func(n goast.Node) bool {
		typeSpec, ok := n.(*goast.TypeSpec)
		if !ok {
			return true
		}
		structType, ok := typeSpec.Type.(*goast.StructType)
		if !ok {
			return true
		}

		def := StructDef{Name: typeSpec.Name.Name}
		for _, astField := range structType.Fields.List {
			if len(astField.Names) == 0 {
				// Embedded type: record for same-file expansion below.
				switch et := astField.Type.(type) {
				case *goast.Ident:
					def.Embeds = append(def.Embeds, et.Name)
				default:
					def.Embeds = append(def.Embeds, typeExprString(astField.Type))
				}
				continue
			}
			for _, nameIdent := range astField.Names {
				field, ok := resolveNamedField(astField, nameIdent.Name, aliases)
				if !ok {
					continue
				}
				def.Fields = append(def.Fields, field)
			}
		}
		defs = append(defs, def)
		return true
	})

	expandEmbeds(defs)
	return defs, nil
}

// importAliases maps local package identifiers to import paths so aliased
// qualified types resolve correctly (`import t "time"` → t.Time == time.Time).
func importAliases(file *goast.File) map[string]string {
	m := make(map[string]string)
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "" {
			p := path
			if i := strings.LastIndex(p, "/"); i >= 0 {
				p = p[i+1:]
			}
			if strings.ContainsAny(p, ".-") {
				continue // versioned/domain paths: package name unreliable here
			}
			name = p
		}
		if name == "_" || name == "." {
			continue
		}
		m[name] = path
	}
	return m
}

type typeCtx struct {
	aliases map[string]string
}

func resolveNamedField(astField *goast.Field, name string, aliases map[string]string) (Field, bool) {
	tags := parseAllTags(astField)

	kind, typeName, elemKind, elemTypeName := resolveTypeCtx(astField.Type, &typeCtx{aliases: aliases})
	if kind == KindUnsupported {
		return Field{}, false
	}

	return Field{
		GoName:       name,
		Tags:         tags,
		Kind:         kind,
		TypeName:     typeName,
		ElemKind:     elemKind,
		ElemTypeName: elemTypeName,
	}, true
}

// ParsePackageName extracts the package name declared in the file.
func ParsePackageName(filename string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.PackageClauseOnly)
	if err != nil {
		return "", fmt.Errorf("ast: failed to read package name from %s: %w", filename, err)
	}
	return file.Name.Name, nil
}

func parseAllTags(astField *goast.Field) map[string]string {
	if astField.Tag == nil {
		return nil
	}
	raw := trimBackticks(astField.Tag.Value)
	return extractAllTagValues(raw)
}

func trimBackticks(s string) string {
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		return s[1 : len(s)-1]
	}
	return s
}

func extractAllTagValues(tag string) map[string]string {
	result := make(map[string]string)
	for tag != "" {
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		i = 0
		for i < len(tag) && tag[i] != ':' {
			i++
		}
		if i >= len(tag) {
			break
		}
		k := tag[:i]
		tag = tag[i+1:]

		if len(tag) == 0 || tag[0] != '"' {
			break
		}
		tag = tag[1:]
		i = 0
		for i < len(tag) && tag[i] != '"' {
			i++
		}
		if i > len(tag) {
			break
		}
		v := tag[:i]
		tag = tag[i+1:]

		result[k] = v
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// ResolveType categorizes a type goast.Expr into a FieldKind. Exported
// for consumers that walk the AST directly (e.g. nitro/json/gen
// route.go, which processes function signatures, not only structs).
func ResolveType(expr goast.Expr) (kind FieldKind, typeName string, elemKind FieldKind, elemTypeName string) {
	return resolveTypeCtx(expr, nil)
}

func resolveType(expr goast.Expr) (kind FieldKind, typeName string, elemKind FieldKind, elemTypeName string) {
	return resolveTypeCtx(expr, nil)
}

func resolveTypeCtx(expr goast.Expr, tc *typeCtx) (kind FieldKind, typeName string, elemKind FieldKind, elemTypeName string) {
	switch t := expr.(type) {
	case *goast.Ident:
		switch t.Name {
		case "string":
			return KindString, t.Name, 0, ""
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64":
			return KindInt, t.Name, 0, ""
		case "float32", "float64":
			return KindFloat, t.Name, 0, ""
		case "bool":
			return KindBool, t.Name, 0, ""
		case "Time", "time.Time":
			return KindTime, t.Name, 0, ""
		default:
			return KindStruct, t.Name, 0, ""
		}
	case *goast.SelectorExpr:
		// Resolve through the file's import table first: an alias
		// (`import t "time"`) must classify t.Time as time.Time.
		if tc != nil && tc.aliases != nil {
			if ident, ok := t.X.(*goast.Ident); ok {
				if tc.aliases[ident.Name] == "time" && t.Sel.Name == "Time" {
					return KindTime, "time.Time", 0, ""
				}
			}
		}
		// Handle time.Time without aliases (X = "time", Sel = "Time")
		if ident, ok := t.X.(*goast.Ident); ok && ident.Name == "time" && t.Sel.Name == "Time" {
			return KindTime, "time.Time", 0, ""
		}
		// pkg.Type (e.g. nitro.StringSlice, nitro.JSONSlice[…])
		return KindStruct, typeExprString(t), 0, ""
	case *goast.IndexExpr:
		// Generic instantiation: JSONSlice[string], nitro.JSON[map[string]any]
		base := typeExprString(t.X)
		idx := typeExprString(t.Index)
		return KindStruct, base + "[" + idx + "]", 0, ""
	case *goast.IndexListExpr:
		base := typeExprString(t.X)
		parts := make([]string, 0, len(t.Indices))
		for _, ix := range t.Indices {
			parts = append(parts, typeExprString(ix))
		}
		return KindStruct, base + "[" + strings.Join(parts, ",") + "]", 0, ""
	case *goast.StarExpr:
		innerKind, innerName, ek, etn := resolveTypeCtx(t.X, tc)
		if innerKind == KindUnsupported || innerName == "" {
			return KindUnsupported, "", 0, ""
		}
		if innerKind == KindStruct {
			return KindStructPtr, "*" + innerName, 0, ""
		}
		return innerKind, "*" + innerName, ek, etn
	case *goast.ArrayType:
		if t.Len != nil {
			return KindUnsupported, "", 0, ""
		}
		ek, etn, _, _ := resolveTypeCtx(t.Elt, tc)
		if ek == KindUnsupported {
			return KindUnsupported, "", 0, ""
		}
		return KindSlice, "[]" + etn, ek, etn
	case *goast.MapType:
		// map[K]V where K must be string
		keyKind, _, _, _ := resolveTypeCtx(t.Key, tc)
		if keyKind != KindString {
			return KindUnsupported, "", 0, ""
		}
		valKind, valName, _, _ := resolveTypeCtx(t.Value, tc)
		if valKind == KindUnsupported {
			return KindUnsupported, "", 0, ""
		}
		typeName := "map[string]" + valName
		return KindMap, typeName, valKind, valName
	default:
		return KindUnsupported, "", 0, ""
	}
}

// expandEmbeds inlines the promoted fields of same-file embedded structs,
// recursively and cycle-safe; fields declared directly on the struct win over
// promoted ones with the same name (Go shadowing semantics).
func expandEmbeds(defs []StructDef) {
	if len(defs) == 0 {
		return
	}
	fieldsOf := make(map[string][]Field, len(defs))
	for i := range defs {
		fieldsOf[defs[i].Name] = defs[i].Fields
	}

	expanded := make(map[string][]Field, len(defs))
	visiting := make(map[string]bool)

	var resolve func(name string) []Field
	resolve = func(name string) []Field {
		if f, ok := expanded[name]; ok {
			return f
		}
		if visiting[name] {
			return nil // inheritance cycle: stop expanding this branch
		}
		visiting[name] = true
		defer delete(visiting, name)

		own, ok := fieldsOf[name]
		if !ok {
			return nil // unknown embed target: nothing to inline
		}
		seen := make(map[string]bool, len(own))
		out := make([]Field, 0, len(own)+len(defs)/2)
		appendUnique := func(fs []Field) {
			for _, f := range fs {
				if seen[f.GoName] {
					continue
				}
				seen[f.GoName] = true
				out = append(out, f)
			}
		}
		for _, d := range defs {
			if d.Name != name {
				continue
			}
			for _, emb := range d.Embeds {
				appendUnique(resolve(emb))
			}
			break
		}
		appendUnique(own)
		expanded[name] = out
		return out
	}

	for i := range defs {
		defs[i].Fields = resolve(defs[i].Name)
		defs[i].Embeds = unresolvedEmbeds(defs[i], fieldsOf)
	}
}

// unresolvedEmbeds keeps only embed targets that could not be expanded
// (not declared in the same file), so callers can still see them.
func unresolvedEmbeds(def StructDef, fieldsOf map[string][]Field) []string {
	var out []string
	for _, emb := range def.Embeds {
		if _, ok := fieldsOf[emb]; !ok {
			out = append(out, emb)
		}
	}
	return out
}

func typeExprString(expr goast.Expr) string {
	switch t := expr.(type) {
	case *goast.Ident:
		return t.Name
	case *goast.SelectorExpr:
		return typeExprString(t.X) + "." + t.Sel.Name
	case *goast.StarExpr:
		return "*" + typeExprString(t.X)
	case *goast.ArrayType:
		if t.Len != nil {
			return "[N]" + typeExprString(t.Elt)
		}
		return "[]" + typeExprString(t.Elt)
	case *goast.MapType:
		return "map[" + typeExprString(t.Key) + "]" + typeExprString(t.Value)
	case *goast.IndexExpr:
		return typeExprString(t.X) + "[" + typeExprString(t.Index) + "]"
	case *goast.IndexListExpr:
		parts := make([]string, 0, len(t.Indices))
		for _, ix := range t.Indices {
			parts = append(parts, typeExprString(ix))
		}
		return typeExprString(t.X) + "[" + strings.Join(parts, ",") + "]"
	case *goast.InterfaceType:
		return "any"
	default:
		return "unknown"
	}
}

// TagValue extracts the value of a specific tag, taking only the part
// before the first comma (e.g. "name,omitempty" -> "name"). Returns
// ok=false if the tag is missing or the value is "-".
func TagValue(f Field, key string) (string, bool) {
	if f.Tags == nil {
		return "", false
	}
	v, ok := f.Tags[key]
	if !ok || v == "" || v == "-" {
		return "", false
	}
	for i := 0; i < len(v); i++ {
		if v[i] == ',' {
			return v[:i], true
		}
	}
	return v, true
}
