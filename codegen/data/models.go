// Package datagen implements Nitro Phase 7: generation of the data
// metamodel used by the Data Access API (LINQ-style).
//
// BENCHMARK-VALIDATED DECISION (see data_proto/): a codegen metamodel
// is ~33% faster than cached reflection (even with a hot sync.Map
// cache), because even a cache hit pays for a Load keyed by
// reflect.Type. Codegen removes that residual cost by turning the
// information into compile-time known values.
//
// Table-name convention: snake_case + plural (e.g. User -> users,
// OrderItem -> order_items), overridable via a struct-level comment
// //nitro:table=custom_name (see table_override.go).
package datagen

import (
	"strings"

	nitroast "github.com/rodrigocelsius/go-nitro/codegen/ast/v2"
)

// Model is a struct ready for data-metamodel generation: table name
// already resolved (convention or override) and columns with column
// names (snake_case convention or `db` tag).
type Model struct {
	StructName string
	TableName  string
	Columns    []Column
}

// Column represents a field mapped to a column.
type Column struct {
	GoName     string
	ColumnName string
	Kind       nitroast.FieldKind
	TypeName   string
	Tags       map[string]string
}

// wellKnownField defines a default field that may be injected automatically.
type wellKnownField struct {
	GoName     string
	ColumnName string
	Kind       nitroast.FieldKind
	TypeName   string
	Nullable   bool
}

// wellKnownFields are the default fields injected automatically if they
// are missing from the user struct. ID is always injected as the first
// field.
var wellKnownFields = []wellKnownField{
	{GoName: "ID", ColumnName: "id", Kind: nitroast.KindStruct, TypeName: "UUID", Nullable: false},
	{GoName: "CreatedAt", ColumnName: "created_at", Kind: nitroast.KindStruct, TypeName: "DateTime", Nullable: false},
	{GoName: "UpdatedAt", ColumnName: "updated_at", Kind: nitroast.KindStruct, TypeName: "DateTime", Nullable: false},
	{GoName: "DeletedAt", ColumnName: "deleted_at", Kind: nitroast.KindStruct, TypeName: "*DateTime", Nullable: true},
}

// ParseModels parses a Go file and returns a Model for each struct
// found. Fields with unsupported types (nested structs, slices) are
// silently ignored — they are not supported as columns in this first
// Phase 7 version.
//
// Default fields (ID string/UUID v7, CreatedAt, UpdatedAt, DeletedAt)
// are injected automatically if missing from the struct. Annotations:
//   - //nitro:notimestamps — do not inject CreatedAt/UpdatedAt
//   - //nitro:nosoftdelete — do not inject DeletedAt (hard DELETE)
//   - //nitro:noid — do not inject ID
//
// At runtime, embed nitro.Model (soft) or nitro.HardModel (hard):
//
//	type Product struct {
//	    nitro.Model
//	    Name string `json:"name" db:"name"`
//	}
//
// Anonymous embeds are NOT expanded by the AST parser: codegen still
// injects id/timestamps/deleted_at columns into the metamodel/migration.
// The Go struct only gains those fields at runtime via the nitro.Model embed.
func ParseModels(filename string) ([]Model, error) {
	structDefs, err := nitroast.ParseFile(filename)
	if err != nil {
		return nil, err
	}

	tableOverrides, err := scanTableOverrides(filename)
	if err != nil {
		return nil, err
	}

	structAnnotations, err := scanStructAnnotations(filename)
	if err != nil {
		return nil, err
	}

	var models []Model
	for _, def := range structDefs {
		model := Model{
			StructName: def.Name,
			TableName:  resolveTableName(def.Name, tableOverrides[def.Name]),
		}

		annotations := structAnnotations[def.Name]

		existingFields := make(map[string]bool)
		for _, f := range def.Fields {
			if !isColumnField(f) {
				continue
			}
			colName, ok := nitroast.TagValue(f, "db")
			if !ok {
				colName = toSnakeCase(f.GoName)
			}
			model.Columns = append(model.Columns, Column{
				GoName:     f.GoName,
				ColumnName: colName,
				Kind:       f.Kind,
				TypeName:   f.TypeName,
				Tags:       f.Tags,
			})
			existingFields[f.GoName] = true
		}

		model.Columns = injectWellKnownFields(model.Columns, existingFields, annotations)

		if len(model.Columns) > 0 {
			models = append(models, model)
		}
	}
	return models, nil
}

func isSupportedColumnKind(k nitroast.FieldKind) bool {
	switch k {
	case nitroast.KindString, nitroast.KindInt, nitroast.KindFloat, nitroast.KindBool, nitroast.KindTime,
		nitroast.KindSlice, nitroast.KindMap:
		return true
	default:
		return false
	}
}

func isColumnField(f nitroast.Field) bool {
	if isSupportedColumnKind(f.Kind) {
		return true
	}
	if f.Kind == nitroast.KindStruct && isJSONTypeName(f.TypeName) {
		return true
	}
	if f.Kind == nitroast.KindStruct && isPortableTypeName(f.TypeName) {
		return true
	}
	if f.Kind == nitroast.KindStructPtr && isPortableTypeName(strings.TrimPrefix(f.TypeName, "*")) {
		return true
	}
	return hasJSONColumnHint(f.Tags)
}

// isPortableTypeName detects nitro portable column types.
func isPortableTypeName(name string) bool {
	base := name
	if i := strings.IndexByte(base, '['); i >= 0 {
		base = base[:i]
	}
	if i := strings.LastIndexByte(base, '.'); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimPrefix(base, "*")
	switch base {
	case "UUID", "Date", "TimeOfDay", "DateTime", "Money", "Measure", "Decimal",
		"Latitude", "Longitude", "LatLng":
		return true
	}
	return false
}

// isJSONTypeName detects nitro.JSON / JSONSlice / StringSlice (with optional package + type args).
func isJSONTypeName(name string) bool {
	base := name
	if i := strings.IndexByte(base, '['); i >= 0 {
		base = base[:i]
	}
	if i := strings.LastIndexByte(base, '.'); i >= 0 {
		base = base[i+1:]
	}
	switch base {
	case "JSON", "JSONSlice", "StringSlice", "Attrs":
		return true
	}
	return false
}

func hasJSONColumnHint(tags map[string]string) bool {
	if tags == nil {
		return false
	}
	if db, ok := tags["db"]; ok {
		for _, p := range strings.Split(db, ",") {
			if strings.EqualFold(strings.TrimSpace(p), "jsonb") || strings.EqualFold(strings.TrimSpace(p), "json") {
				return true
			}
		}
	}
	blinq := tags["blinq"]
	for _, p := range strings.Split(blinq, ",") {
		p = strings.TrimSpace(p)
		if i := strings.IndexByte(p, '='); i >= 0 {
			p = p[:i]
		}
		if strings.EqualFold(p, "jsonb") || strings.EqualFold(p, "json") {
			return true
		}
	}
	return false
}

// resolveTableName applies the convention (snake_case + plural) or uses
// the explicit override, if present.
func resolveTableName(structName, override string) string {
	if override != "" {
		return override
	}
	return tableNameFromStruct(structName)
}

// ParsePackageName extracts the package name declared in the file.
func ParsePackageName(filename string) (string, error) {
	return nitroast.ParsePackageName(filename)
}
