package validator

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/rodrigocelsius/go-nitro/usecases/v2/jsontag"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
)

// ApplyDefaults fills JSON-omitted fields from the `default:"…"` tag
// (or `validate:"default=…"`).
//
// raw is JSON already decoded as map[string]any / []any (key presence).
// So `"active": false` is not overwritten by default:"true" — only missing keys.
//
// Tipos: bool, string, ints, uints, floats.
func ApplyDefaults(v any, raw any) {
	if v == nil {
		return
	}
	rv := reflect.ValueOf(v)
	applyDefaultsValue(rv, raw)
}

func applyDefaultsValue(rv reflect.Value, raw any) {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return
	}

	switch rv.Kind() {
	case reflect.Struct:
		rawMap, _ := raw.(map[string]any)
		applyDefaultsStruct(rv, rawMap)
	case reflect.Slice, reflect.Array:
		rawSlice, ok := asSlice(raw)
		if !ok {
			return
		}
		for i := 0; i < rv.Len() && i < len(rawSlice); i++ {
			applyDefaultsValue(rv.Index(i), rawSlice[i])
		}
		// elements beyond raw: apply defaults with no map (all "absent")
		for i := len(rawSlice); i < rv.Len(); i++ {
			applyDefaultsValue(rv.Index(i), nil)
		}
	}
}

func applyDefaultsStruct(rv reflect.Value, raw map[string]any) {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" {
			continue
		}
		fv := rv.Field(i)
		jsonName := jsonFieldName(f)
		if jsonName == "-" {
			continue
		}

		var childRaw any
		present := false
		if raw != nil {
			childRaw, present = lookupRawKey(raw, wireNames(f)...)
		}

		switch fv.Kind() {
		case reflect.Struct:
			if f.Type.PkgPath() == "time" && f.Type.Name() == "Time" {
				if !present {
					applyDefaultScalar(fv, defaultTag(f))
				}
				continue
			}
			childMap, _ := childRaw.(map[string]any)
			if present {
				applyDefaultsStruct(fv, childMap)
			} else {
				applyDefaultsStruct(fv, nil)
			}
		case reflect.Pointer:
			if !present {
				if def := defaultTag(f); def != "" && fv.IsNil() && fv.CanSet() {
					elem := reflect.New(fv.Type().Elem())
					if applyDefaultScalar(elem.Elem(), def) {
						fv.Set(elem)
					}
				}
				continue
			}
			if !fv.IsNil() {
				applyDefaultsValue(fv, childRaw)
			}
		case reflect.Slice, reflect.Array:
			if present {
				applyDefaultsValue(fv, childRaw)
			}
		default:
			if !present {
				applyDefaultScalar(fv, defaultTag(f))
			}
		}
	}
}

// wireNames returns JSON / form / query tag names used to detect key presence
// (BodyParser form/query builds presence maps with these keys).
// fieldNameKey identifies a struct field definition; names derived from it
// are pure functions of (name, tag), so they are computed once (bounded by the
// number of fields in the program, never by request input).
type fieldNameKey struct {
	name string
	tag  reflect.StructTag
}

var (
	jsonNameCache sync.Map // fieldNameKey → string
	wireNameCache sync.Map // fieldNameKey → []string (read-only)
)

// wireNames lists the keys a field may arrive under (cached; do not mutate).
func wireNames(f reflect.StructField) []string {
	key := fieldNameKey{f.Name, f.Tag}
	if v, ok := wireNameCache.Load(key); ok {
		return v.([]string)
	}
	out := computeWireNames(f)
	wireNameCache.Store(key, out)
	return out
}

func computeWireNames(f reflect.StructField) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || name == "-" {
			return
		}
		if i := strings.IndexByte(name, ','); i >= 0 {
			name = name[:i]
		}
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	add(f.Tag.Get("json"))
	add(f.Tag.Get("form"))
	add(f.Tag.Get("query"))
	add(jsonFieldName(f))
	return out
}

func lookupRawKey(raw map[string]any, keys ...string) (any, bool) {
	for _, k := range keys {
		if v, ok := raw[k]; ok {
			return v, true
		}
	}
	return nil, false
}

func defaultTag(f reflect.StructField) string {
	if v := strings.TrimSpace(f.Tag.Get("default")); v != "" {
		return v
	}
	// validate:"required,default=true" ou "default=true"
	tag := f.Tag.Get("validate")
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "default=") {
			return strings.TrimPrefix(part, "default=")
		}
		if part == "default" {
			return "true" // default with no arg → true (useful for bool)
		}
	}
	return ""
}

func applyDefaultScalar(fv reflect.Value, def string) bool {
	if def == "" || !fv.CanSet() {
		return false
	}
	switch fv.Kind() {
	case reflect.Bool:
		b, err := strconv.ParseBool(def)
		if err != nil {
			return false
		}
		fv.SetBool(b)
		return true
	case reflect.String:
		fv.SetString(def)
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(def, 10, 64)
		if err != nil {
			return false
		}
		fv.SetInt(n)
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(def, 10, 64)
		if err != nil {
			return false
		}
		fv.SetUint(n)
		return true
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(def, 64)
		if err != nil {
			return false
		}
		fv.SetFloat(n)
		return true
	default:
		return false
	}
}

func asSlice(raw any) ([]any, bool) {
	switch s := raw.(type) {
	case []any:
		return s, true
	case nil:
		return nil, false
	default:
		rv := reflect.ValueOf(raw)
		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			return nil, false
		}
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out, true
	}
}

// UnmarshalJSONDefaults runs jsontag.Unmarshal (json.Unmarshal + the
// default-name fallback for untagged fields) + ApplyDefaults with
// omitted-key detection.
func UnmarshalJSONDefaults(data []byte, v any) error {
	// Fast path: most request types declare no `default:` tags, so the
	// second (map[string]any) decode used for key-presence is wasted work.
	if v != nil && !typeNeedsDefaults(reflect.TypeOf(v)) {
		if err := jsontag.Unmarshal(data, v); err != nil {
			return err
		}
		ApplyTransforms(v)
		return nil
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := jsontag.Unmarshal(data, v); err != nil {
		return err
	}
	// encoding/json uses map[string]any; numbers become float64 — fine for presence.
	ApplyDefaults(v, normalizeRaw(raw))
	// Pipeline: default → transform → validate (validate runs in BodyJSON/Parse).
	ApplyTransforms(v)
	return nil
}

// UnmarshalJSONPatch unmarshals onto an existing value (PATCH merge, with
// the default-name fallback for untagged fields) + transforms — **without**
// ApplyDefaults (does not zero/overwrite omitted fields).
func UnmarshalJSONPatch(data []byte, v any) error {
	if err := jsontag.Unmarshal(data, v); err != nil {
		return err
	}
	ApplyTransforms(v)
	return nil
}

// ApplyTransforms applies `transform:"…"` tags (titlecase, digits, upper, …).
func ApplyTransforms(v any) {
	transform.Apply(v)
}

func normalizeRaw(raw any) any {
	switch t := raw.(type) {
	case map[string]any:
		return t
	case []any:
		for i, el := range t {
			t[i] = normalizeRaw(el)
		}
		return t
	default:
		return raw
	}
}

// ParseDefaultArg validates a default= arg (exported for tests).
func ParseDefaultArg(arg string) (string, error) {
	if arg == "" {
		return "", fmt.Errorf("empty default")
	}
	return arg, nil
}

var needsDefaultsCache sync.Map // reflect.Type -> bool

// typeNeedsDefaults reports whether ApplyDefaults could change a value of
// type t (any reachable struct field carries a default). Cached per type.
func typeNeedsDefaults(t reflect.Type) bool {
	if cached, ok := needsDefaultsCache.Load(t); ok {
		return cached.(bool)
	}
	res := walkNeedsDefaults(t, map[reflect.Type]bool{})
	needsDefaultsCache.Store(t, res)
	return res
}

func walkNeedsDefaults(t reflect.Type, seen map[reflect.Type]bool) bool {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return false
	}
	seen[t] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if defaultTag(f) != "" || walkNeedsDefaults(f.Type, seen) {
			return true
		}
	}
	return false
}
