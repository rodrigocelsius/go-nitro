// Package jsontag lets model fields omit the `json` tag entirely: the
// wire key then defaults to snake_case of the Go field name (same
// convention used for the `db` column — see data/adapter/pgx and
// blinq/scan). Fields that declare an explicit `json:"..."` tag are
// untouched; `json:"-"` keeps excluding the field.
//
// Go's encoding/json cannot be told to rename untagged fields (it always
// marshals/unmarshals them under the exact Go field name), so this package
// wraps it: for types that have at least one field relying on the default
// name (recursively, including anonymous embeds and nested structs/slices),
// it marshals/unmarshals normally and then rewrites just those JSON keys.
// Types that are fully tagged (the common case today) take the original
// json.Marshal/json.Unmarshal path with zero extra cost — the per-type
// "needs rename" check is computed once and cached.
package jsontag

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"

	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
)

var needsRenameCache sync.Map // reflect.Type -> bool

// NeedsRename reports whether v's type has at least one field (recursively)
// relying on the default (untagged) name. Callers on a hot path (e.g. the
// HTTP response encoder) can use this to skip the rename machinery entirely
// for fully-tagged types — which is every model in the framework today —
// and fall through to a plain, pooled json.Marshal/Encoder call.
func NeedsRename(v any) bool {
	t, ok := structTypeOf(v)
	return ok && needsRenameType(t)
}

// Marshal behaves like json.Marshal, except fields without an explicit
// `json` tag are emitted under snake_case(FieldName) instead of the raw Go
// field name.
func Marshal(v any) ([]byte, error) {
	t, ok := structTypeOf(v)
	if !ok || !needsRenameType(t) {
		return json.Marshal(v)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return renameValue(raw, t, toDefault)
}

// Unmarshal behaves like json.Unmarshal, except it also accepts
// snake_case(FieldName) as the wire key for fields without an explicit
// `json` tag (in addition to whatever an explicit tag already accepts).
func Unmarshal(data []byte, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return json.Unmarshal(data, v)
	}
	t := rv.Type().Elem()
	if t.Kind() != reflect.Struct || !needsRenameType(t) {
		return json.Unmarshal(data, v)
	}
	converted, err := renameValue(data, t, toGo)
	if err != nil {
		// Fall back to the stdlib call so the caller gets the real parse error.
		return json.Unmarshal(data, v)
	}
	return json.Unmarshal(converted, v)
}

func structTypeOf(v any) (reflect.Type, bool) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, false
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t, t.Kind() == reflect.Struct
}

// direction picks which key becomes the map key after rename: the Go-side
// key (emitted by encoding/json for untagged fields, i.e. f.Name) or the
// default wire key (snake_case(f.Name)).
type direction int

const (
	toDefault direction = iota // Go field name -> default wire key (Marshal)
	toGo                       // default wire key -> Go field name (Unmarshal)
)

// needsRenameType reports whether t (or anything reachable from it through
// anonymous embeds, struct fields, or slice/array elements) has at least one
// field relying on the default (untagged) name. Memoized per type.
func needsRenameType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		return needsRenameStruct(t)
	case reflect.Slice, reflect.Array:
		return needsRenameType(t.Elem())
	default:
		return false
	}
}

func needsRenameStruct(t reflect.Type) bool {
	if v, ok := needsRenameCache.Load(t); ok {
		return v.(bool)
	}
	// Cycle guard: assume false while computing (self-referential model
	// structs are not a real-world case here).
	needsRenameCache.Store(t, false)

	result := false
	for i := 0; i < t.NumField() && !result; i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		if f.Anonymous && tag == "" {
			ft := f.Type
			for ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				if needsRenameStruct(ft) {
					result = true
				}
				continue
			}
		}
		if !hasExplicitJSONName(tag) {
			result = true
			continue
		}
		if needsRenameType(f.Type) {
			result = true
		}
	}
	needsRenameCache.Store(t, result)
	return result
}

// wireKeyOf returns the key an explicitly tagged field already uses (both
// for Marshal and Unmarshal); callers must only use this when tag has an
// explicit name — see hasExplicitJSONName.
func wireKeyOf(f reflect.StructField, tag string) string {
	if hasExplicitJSONName(tag) {
		return strings.SplitN(tag, ",", 2)[0]
	}
	return f.Name
}

func hasExplicitJSONName(tag string) bool {
	if tag == "" {
		return false
	}
	name := strings.SplitN(tag, ",", 2)[0]
	return name != ""
}

// renameValue dispatches by kind: struct (rewrite its own map, recursing
// into fields that need it) or slice/array (recurse per element).
func renameValue(raw json.RawMessage, t reflect.Type, dir direction) (json.RawMessage, error) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		return renameStruct(raw, t, dir)
	case reflect.Slice, reflect.Array:
		return renameSlice(raw, t.Elem(), dir)
	default:
		return raw, nil
	}
}

func renameSlice(raw json.RawMessage, elemType reflect.Type, dir direction) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return raw, nil
	}
	if !needsRenameType(elemType) {
		return raw, nil
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return raw, err
	}
	for i, el := range elems {
		renamed, err := renameValue(el, elemType, dir)
		if err != nil {
			return raw, err
		}
		elems[i] = renamed
	}
	return json.Marshal(elems)
}

// renameStruct walks t's fields (flattening anonymous embeds, as
// encoding/json does) and moves values between the Go-name key and the
// default wire key, recursing into nested values whose type also needs
// renaming. Explicitly tagged fields are left in place (only their nested
// value may still be rewritten).
func renameStruct(raw json.RawMessage, t reflect.Type, dir direction) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return raw, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw, err
	}
	out := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		out[k] = v
	}
	handled := map[string]bool{}

	var walk func(rt reflect.Type)
	walk = func(rt reflect.Type) {
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if f.PkgPath != "" && !f.Anonymous {
				continue
			}
			tag := f.Tag.Get("json")
			if tag == "-" {
				continue
			}
			if f.Anonymous && tag == "" {
				ft := f.Type
				for ft.Kind() == reflect.Ptr {
					ft = ft.Elem()
				}
				if ft.Kind() == reflect.Struct {
					walk(ft)
					continue
				}
			}

			// currentKey is whatever key already holds this field's value in
			// `out` before any rename: explicit tags are left untouched in
			// both directions (they're already correct for encoding/json);
			// only the untagged (default-name) case needs to move between
			// the Go field name and its snake_case default.
			currentKey := wireKeyOf(f, tag)
			if !hasExplicitJSONName(tag) {
				goKey := f.Name
				defaultKey := transform.SnakeCase(f.Name)
				fromKey, toKey := goKey, defaultKey
				if dir == toGo {
					fromKey, toKey = defaultKey, goKey
				}
				currentKey = toKey
				if !handled[fromKey] {
					if v, ok := m[fromKey]; ok {
						delete(out, fromKey)
						out[toKey] = v
						handled[fromKey] = true
					}
				}
			}

			if v, ok := out[currentKey]; ok && needsRenameType(f.Type) {
				if renamed, err := renameValue(v, f.Type, dir); err == nil {
					out[currentKey] = renamed
				}
			}
		}
	}
	walk(t)

	return json.Marshal(out)
}
