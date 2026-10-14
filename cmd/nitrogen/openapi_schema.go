package main

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	nitroast "github.com/rodrigocelsius/go-nitro/codegen/ast/v2"
	openapi "github.com/rodrigocelsius/go-nitro/codegen/openapi/v2"
)

// schemaBuilder turns the project's Go structs into OpenAPI component schemas
// following Nitro's wire conventions: JSON names default to snake_case of the
// Go field (explicit json tags win), validate:"required" marks required, and
// ID/CreatedAt/UpdatedAt/DeletedAt plus readonly/computed fields are
// server-owned (readOnly in the resource, absent from <T>Input).
type schemaBuilder struct {
	defs  map[string]nitroast.StructDef
	gen   *openapi.Generator
	added map[string]bool
}

func newSchemaBuilder(root string, gen *openapi.Generator) *schemaBuilder {
	b := &schemaBuilder{defs: map[string]nitroast.StructDef{}, gen: gen, added: map[string]bool{}}
	fromModelDir := map[string]bool{}
	for _, f := range collectGoFiles(root) {
		defs, err := nitroast.ParseFile(f)
		if err != nil {
			continue
		}
		inModel := strings.Contains(filepath.ToSlash(f), "/model")
		for _, d := range defs {
			// Same type name in several packages: prefer the model package.
			if _, dup := b.defs[d.Name]; dup && (fromModelDir[d.Name] || !inModel) {
				continue
			}
			b.defs[d.Name] = d
			fromModelDir[d.Name] = inModel
		}
	}
	return b
}

// resourceSchemas names the components documenting one Resource model.
type resourceSchemas struct {
	full, input, patch, list string
}

// resource registers <T>, <T>Input (create body), <T>Patch (PATCH body: same
// fields, none required) and <T>List (paginated envelope), or ok=false when
// the model struct is not found in the project.
func (b *schemaBuilder) resource(model string) (resourceSchemas, bool) {
	if _, found := b.defs[model]; !found {
		return resourceSchemas{}, false
	}
	rs := resourceSchemas{model, model + "Input", model + "Patch", model + "List"}
	b.add(model)
	if !b.added[rs.input] {
		b.added[rs.input] = true
		b.gen.AddSchema(rs.input, b.object(model, true))
		patch := b.object(model, true)
		patch.Required = nil
		b.gen.AddSchema(rs.patch, patch)
	}
	list := rs.list
	if !b.added[list] {
		b.added[list] = true
		b.gen.AddSchema(list, openapi.ObjectSchema(map[string]*openapi.Schema{
			"items":       {Type: "array", Items: openapi.RefSchema(model)},
			"total":       {Type: "integer", Format: "int64"},
			"page":        {Type: "integer"},
			"per_page":    {Type: "integer"},
			"total_pages": {Type: "integer"},
		}, "items", "total", "page", "per_page", "total_pages"))
	}
	return rs, true
}

// add registers components.schemas[name] for a project struct (once).
func (b *schemaBuilder) add(name string) {
	if b.added[name] {
		return
	}
	b.added[name] = true // before recursing: self-referencing structs
	b.gen.AddSchema(name, b.object(name, false))
}

type schemaField struct {
	name        string
	schema      *openapi.Schema
	required    bool
	serverOwned bool
}

func (b *schemaBuilder) object(name string, input bool) *openapi.Schema {
	props := map[string]*openapi.Schema{}
	var required []string
	for _, f := range b.fields(name, map[string]bool{}) {
		if input && f.serverOwned {
			continue
		}
		s := f.schema
		if !input && f.serverOwned {
			cp := *s
			cp.ReadOnly = true
			s = &cp
		}
		props[f.name] = s
		if f.required {
			required = append(required, f.name)
		}
	}
	sort.Strings(required)
	return openapi.ObjectSchema(props, required...)
}

// fields lists the JSON fields of a struct, expanding embeds: nitro.Model /
// HardModel / TenantModel by convention, and project structs by recursion.
func (b *schemaBuilder) fields(name string, visiting map[string]bool) []schemaField {
	def, ok := b.defs[name]
	if !ok || visiting[name] {
		return nil
	}
	visiting[name] = true
	defer delete(visiting, name)

	var out []schemaField
	seen := map[string]bool{}
	push := func(fs ...schemaField) {
		for _, f := range fs {
			if !seen[f.name] {
				seen[f.name] = true
				out = append(out, f)
			}
		}
	}
	// Own fields first: they shadow promoted ones.
	for _, f := range def.Fields {
		if sf, ok := b.field(f); ok {
			push(sf)
		}
	}
	for _, emb := range def.Embeds {
		push(b.embedFields(emb, visiting)...)
	}
	return out
}

func (b *schemaBuilder) embedFields(emb string, visiting map[string]bool) []schemaField {
	emb = strings.TrimPrefix(emb, "*")
	base := lastSegment(emb)
	if strings.Contains(emb, ".") || b.defs[base].Name == "" {
		switch base {
		case "Model", "HardModel", "TenantModel":
			fs := []schemaField{
				{name: "id", schema: &openapi.Schema{Type: "string", Format: "uuid"}, serverOwned: true},
				{name: "created_at", schema: &openapi.Schema{Type: "string", Format: "date-time"}, serverOwned: true},
				{name: "updated_at", schema: &openapi.Schema{Type: "string", Format: "date-time"}, serverOwned: true},
			}
			if base != "HardModel" {
				fs = append(fs, schemaField{name: "deleted_at", schema: &openapi.Schema{Type: "string", Format: "date-time", Nullable: true}, serverOwned: true})
			}
			if base == "TenantModel" {
				fs = append(fs, schemaField{name: "tenant_id", schema: &openapi.Schema{Type: "string", Format: "uuid"}, serverOwned: true})
			}
			return fs
		}
	}
	return b.fields(base, visiting)
}

func (b *schemaBuilder) field(f nitroast.Field) (schemaField, bool) {
	if f.GoName == "" || f.GoName[0] < 'A' || f.GoName[0] > 'Z' {
		return schemaField{}, false
	}
	name := wireSnake(f.GoName)
	if tag, ok := f.Tags["json"]; ok {
		n := strings.Split(tag, ",")[0]
		if n == "-" {
			return schemaField{}, false
		}
		if n != "" {
			name = n
		}
	}
	s := b.schemaFor(f.Kind, f.TypeName, f.ElemKind, f.ElemTypeName)
	required := false
	for _, rule := range strings.Split(f.Tags["validate"], ",") {
		if strings.TrimSpace(rule) == "required" {
			required = true
		}
	}
	switch f.GoName {
	case "ID", "CreatedAt", "UpdatedAt", "DeletedAt":
		return schemaField{name: name, schema: s, serverOwned: true}, true
	}
	owned := f.Tags["readonly"] == "true" || f.Tags["computed"] == "true"
	return schemaField{name: name, schema: s, required: required && !owned, serverOwned: owned}, true
}

func (b *schemaBuilder) schemaFor(kind nitroast.FieldKind, typeName string, elemKind nitroast.FieldKind, elemType string) *openapi.Schema {
	nullable := strings.HasPrefix(typeName, "*")
	t := strings.TrimPrefix(typeName, "*")
	var s *openapi.Schema
	switch kind {
	case nitroast.KindString:
		s = &openapi.Schema{Type: "string"}
	case nitroast.KindInt:
		s = &openapi.Schema{Type: "integer"}
		if strings.HasSuffix(t, "64") {
			s.Format = "int64"
		} else if strings.HasSuffix(t, "32") {
			s.Format = "int32"
		}
	case nitroast.KindFloat:
		s = &openapi.Schema{Type: "number", Format: "double"}
		if t == "float32" {
			s.Format = "float"
		}
	case nitroast.KindBool:
		s = &openapi.Schema{Type: "boolean"}
	case nitroast.KindTime:
		s = &openapi.Schema{Type: "string", Format: "date-time"}
	case nitroast.KindSlice:
		if elemType == "byte" || elemType == "uint8" {
			s = &openapi.Schema{Type: "string", Format: "byte"}
		} else {
			s = &openapi.Schema{Type: "array", Items: b.schemaFor(elemKind, elemType, 0, "")}
		}
	case nitroast.KindMap:
		s = &openapi.Schema{Type: "object"}
	case nitroast.KindStruct, nitroast.KindStructPtr:
		s = b.namedSchema(t)
	default:
		s = &openapi.Schema{}
	}
	if nullable && s.Ref == "" {
		s.Nullable = true
	}
	return s
}

// namedSchema maps named types: Nitro portable types by their JSON encoding,
// project structs to $ref components, anything else to a free-form object.
func (b *schemaBuilder) namedSchema(t string) *openapi.Schema {
	base, arg := t, ""
	if i := strings.Index(t, "["); i >= 0 && strings.HasSuffix(t, "]") {
		base, arg = t[:i], t[i+1:len(t)-1]
	}
	switch lastSegment(base) {
	case "UUID":
		return &openapi.Schema{Type: "string", Format: "uuid"}
	case "DateTime", "Time":
		return &openapi.Schema{Type: "string", Format: "date-time"}
	case "Date":
		return &openapi.Schema{Type: "string", Format: "date"}
	case "TimeOfDay":
		return &openapi.Schema{Type: "string", Format: "time"}
	case "Decimal":
		return &openapi.Schema{Type: "string", Format: "decimal"}
	case "Latitude", "Longitude":
		return &openapi.Schema{Type: "number", Format: "double"}
	case "StringSlice":
		return &openapi.Schema{Type: "array", Items: &openapi.Schema{Type: "string"}}
	case "JSONSlice":
		return &openapi.Schema{Type: "array", Items: b.namedOrScalar(arg)}
	case "JSON", "Attrs", "Money", "Measure", "LatLng":
		return &openapi.Schema{Type: "object"}
	case "NullString":
		return &openapi.Schema{Type: "string", Nullable: true}
	case "NullInt64", "NullInt32":
		return &openapi.Schema{Type: "integer", Nullable: true}
	case "NullFloat64":
		return &openapi.Schema{Type: "number", Nullable: true}
	case "NullBool":
		return &openapi.Schema{Type: "boolean", Nullable: true}
	case "NullTime":
		return &openapi.Schema{Type: "string", Format: "date-time", Nullable: true}
	}
	if !strings.Contains(base, ".") {
		if _, ok := b.defs[base]; ok {
			b.add(base)
			return openapi.RefSchema(base)
		}
	}
	return &openapi.Schema{Type: "object"}
}

// namedOrScalar resolves a generic type argument (JSONSlice[T]).
func (b *schemaBuilder) namedOrScalar(t string) *openapi.Schema {
	switch t {
	case "string":
		return &openapi.Schema{Type: "string"}
	case "int", "int32", "int64", "uint", "uint32", "uint64":
		return &openapi.Schema{Type: "integer"}
	case "float32", "float64":
		return &openapi.Schema{Type: "number"}
	case "bool":
		return &openapi.Schema{Type: "boolean"}
	case "", "any":
		return &openapi.Schema{}
	}
	return b.namedSchema(t)
}

// wireSnake is the default JSON key of an untagged field — the same rule as
// the runtime (usecases/transform.SnakeCase): CustomerName → customer_name,
// UserID → user_id, Address2Line → address2_line.
func wireSnake(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) || unicode.IsDigit(prev) ||
					(unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
