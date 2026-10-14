// Package gen implements Nitro's Phase 4 JSON code generator.
//
// Core idea: instead of using runtime reflection (as encoding/json
// does) to discover struct fields, tags, and types, Nitro parses
// SOURCE at build time with go/ast (the same parser the Go compiler
// uses internally) and generates a _gen.go file with explicit
// MarshalJSON/UnmarshalJSON functions for each marked struct.
//
// From Phase 7 onward, AST parsing itself (struct/field/type/tag
// extraction) lives in the shared nitro/gen/ast package, also reused
// by the Data Access API generator (nitro/data/gen). This file keeps
// the previous public names (FieldKind, StructDef, ParseFile, etc.)
// via aliases/wrappers so Phase 4/5 code and tests that already
// depended on them keep compiling.
package gen

import (
	nitroast "github.com/rodrigocelsius/go-nitro/codegen/ast/v2"
)

// FieldKind, StructDef and the Kind* constants are direct aliases of
// the shared-package types — Phase 4/5 code keeps working with no
// import or usage change.
type FieldKind = nitroast.FieldKind

const (
	KindString      = nitroast.KindString
	KindInt         = nitroast.KindInt
	KindFloat       = nitroast.KindFloat
	KindBool        = nitroast.KindBool
	KindStruct      = nitroast.KindStruct
	KindStructPtr   = nitroast.KindStructPtr
	KindSlice       = nitroast.KindSlice
	KindMap         = nitroast.KindMap
	KindTime        = nitroast.KindTime
	KindUnsupported = nitroast.KindUnsupported
)

// Field is specific to json/gen (keeps JSONName, which is Phase 4
// applying the "json" tag convention on top of the shared generic Field).
type Field struct {
	GoName       string
	JSONName     string
	Kind         FieldKind
	TypeName     string
	ElemKind     FieldKind
	ElemTypeName string
}

// StructDef is specific to json/gen, with Fields already filtered/
// converted to the format used by the Marshal/Unmarshal templates.
type StructDef struct {
	Name   string
	Fields []Field
}

// ParseFile parses a Go file with the shared AST parser, then
// filters/converts fields into the json/gen format. A field is excluded
// only via an explicit `json:"-"` tag; fields with no `json` tag at all
// (or an empty name, e.g. `json:",omitempty"`) are included under the
// default wire name — snake_case(GoName) — so the generated AppendJSON
// fast path stays consistent with the runtime fallback (see
// usecases/jsontag) for models that omit tags.
func ParseFile(filename string) ([]StructDef, error) {
	rawDefs, err := nitroast.ParseFile(filename)
	if err != nil {
		return nil, err
	}

	var defs []StructDef
	for _, rawDef := range rawDefs {
		def := StructDef{Name: rawDef.Name}
		for _, rawField := range rawDef.Fields {
			if raw, hasTag := rawField.Tags["json"]; hasTag && raw == "-" {
				continue
			}
			jsonName, ok := nitroast.TagValue(rawField, "json")
			if !ok || jsonName == "" {
				jsonName = toSnakeCase(rawField.GoName)
			}
			def.Fields = append(def.Fields, Field{
				GoName:       rawField.GoName,
				JSONName:     jsonName,
				Kind:         rawField.Kind,
				TypeName:     rawField.TypeName,
				ElemKind:     rawField.ElemKind,
				ElemTypeName: rawField.ElemTypeName,
			})
		}
		defs = append(defs, def)
	}
	return defs, nil
}

// ParsePackageName delegates directly to the shared package.
func ParsePackageName(filename string) (string, error) {
	return nitroast.ParsePackageName(filename)
}
