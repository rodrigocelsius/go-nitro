package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
)

// ValidateStruct validates a struct using validate tags.
//
// Supported rules: required, omitempty, dive, min, max, len, in, not_in,
// email, oneof, eq, ne, gt, gte, lt, lte, uuid, uuid4, uuid7, url, uri,
// alpha, alphanum, numeric, startswith, endswith, contains, excludes, datetime,
// required_if, required_unless, required_with, eqfield, nefield, gtfield, gtefield,
// ltfield, ltefield, unique, ip, ipv4, ipv6, cidr, e164 — aliases: same, eq_field
// (= eqfield) — plus rules registered via RegisterValidator / NewValidator().Register.
//
// Nested structs are validated recursively (prefix "address.street").
// With "dive", each slice/array element is validated (prefix "tags[0]").
// With "omitempty", a zero value skips remaining rules (except required) and
// does not descend into nested/dive.
//
// Models may implement Validate() error for business rules after tags.
//
// Note: required on bool treats false as absent (zero-value). Prefer *bool
// when false is a valid value distinct from "not sent".
func ValidateStruct(v any) domain.ValidationErrors {
	return DefaultValidator().Validate(v)
}

func validateValue(prefix string, rv reflect.Value, root any, reg *Registry, opts Options) domain.ValidationErrors {
	rv = derefValue(rv)
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return nil
	}
	if reg == nil {
		reg = DefaultRegistry()
	}
	if root == nil && rv.CanInterface() {
		root = rv.Interface()
	}

	meta := lookupTypeMeta(rv.Type())
	var errs domain.ValidationErrors

	for _, cf := range meta.fields {
		field := cf.field
		fieldValue := rv.Field(cf.index)
		path := joinPath(prefix, cf.jsonName)

		if !cf.hasTag {
			errs = append(errs, validateNested(path, fieldValue, root, reg, opts)...)
			if opts.StopOnFirstError && len(errs) > 0 {
				return errs
			}
			continue
		}

		if cf.meta.required && isZero(fieldValue) {
			var valueAny any
			if fieldValue.IsValid() && fieldValue.CanInterface() {
				valueAny = fieldValue.Interface()
			}
			errs = append(errs, &domain.ValidationError{
				Field:   path,
				Message: ResolveFieldMessage(field, path, "required", "is required", "", valueAny),
				Rule:    "required",
			})
			if opts.StopOnFirstError {
				return errs
			}
		}

		for _, r := range cf.fieldRules {
			if !isConditionalRequired(r.name) {
				continue
			}
			if err := applyRule(reg, root, path, field, fieldValue, rv, r.name, r.arg, opts); err != nil {
				errs = append(errs, err)
				if opts.StopOnFirstError {
					return errs
				}
			}
		}

		if cf.meta.omitempty && isZero(fieldValue) {
			continue
		}

		for _, r := range cf.fieldRules {
			if isConditionalRequired(r.name) {
				continue
			}
			if err := applyRule(reg, root, path, field, fieldValue, rv, r.name, r.arg, opts); err != nil {
				errs = append(errs, err)
				if opts.StopOnFirstError {
					return errs
				}
			}
		}

		if cf.meta.dive {
			errs = append(errs, validateDive(path, field, fieldValue, cf.diveRules, root, reg, opts)...)
		} else {
			errs = append(errs, validateNested(path, fieldValue, root, reg, opts)...)
		}
		if opts.StopOnFirstError && len(errs) > 0 {
			return errs
		}
	}

	return errs
}

func validateNested(path string, value reflect.Value, root any, reg *Registry, opts Options) domain.ValidationErrors {
	v := derefValue(value)
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Struct {
		return validateValue(path, v, root, reg, opts)
	}
	return nil
}

func validateDive(path string, field reflect.StructField, value reflect.Value, diveRules []validateRule, root any, reg *Registry, opts Options) domain.ValidationErrors {
	v := derefValue(value)
	if !v.IsValid() {
		return nil
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		var errs domain.ValidationErrors
		for i := 0; i < v.Len(); i++ {
			elemPath := fmt.Sprintf("%s[%d]", path, i)
			elem := v.Index(i)
			for _, r := range diveRules {
				if err := applyRule(reg, root, elemPath, field, elem, reflect.Value{}, r.name, r.arg, opts); err != nil {
					errs = append(errs, err)
					if opts.StopOnFirstError {
						return errs
					}
				}
			}
			errs = append(errs, validateNested(elemPath, elem, root, reg, opts)...)
			if opts.StopOnFirstError && len(errs) > 0 {
				return errs
			}
		}
		return errs
	default:
		var errs domain.ValidationErrors
		for _, r := range diveRules {
			if err := applyRule(reg, root, path, field, v, reflect.Value{}, r.name, r.arg, opts); err != nil {
				errs = append(errs, err)
				if opts.StopOnFirstError {
					return errs
				}
			}
		}
		errs = append(errs, validateNested(path, v, root, reg, opts)...)
		return errs
	}
}

// jsonFieldName resolves the wire key for error labels/paths and presence
// detection: explicit `json` tag → snake_case(field name) (same default used
// when the tag is omitted entirely — see usecases/jsontag).
func jsonFieldName(field reflect.StructField) string {
	key := fieldNameKey{field.Name, field.Tag}
	if v, ok := jsonNameCache.Load(key); ok {
		return v.(string)
	}
	name := transform.SnakeCase(field.Name)
	if jsonTag := field.Tag.Get("json"); jsonTag != "" {
		if n, _, _ := strings.Cut(jsonTag, ","); n != "" && n != "-" {
			name = n
		}
	}
	jsonNameCache.Store(key, name)
	return name
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func derefValue(v reflect.Value) reflect.Value {
	for v.IsValid() && v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return v
		}
		v = v.Elem()
	}
	return v
}

func isConditionalRequired(rule string) bool {
	switch rule {
	case "required_if", "required_unless", "required_with":
		return true
	}
	return false
}

func applyRule(reg *Registry, root any, path string, field reflect.StructField, value, parent reflect.Value, rule, arg string, opts Options) *domain.ValidationError {
	if reg == nil {
		reg = DefaultRegistry()
	}
	fn, ok := reg.Lookup(rule)
	if !ok {
		if opts.StrictUnknown {
			msg := "unknown validation rule: " + rule
			return &domain.ValidationError{
				Field:   path,
				Message: ResolveFieldMessage(field, path, "unknown", msg, rule, nil),
				Rule:    "unknown",
			}
		}
		return nil
	}

	var parentAny any
	if parent.IsValid() && parent.CanInterface() {
		parentAny = parent.Interface()
	}
	var valueAny any
	if value.IsValid() && value.CanInterface() {
		valueAny = value.Interface()
	}

	jsonName := jsonFieldName(field)
	if jsonName == "" {
		jsonName = path
	}
	ctx := domain.FieldContext{
		Root:       root,
		Parent:     parentAny,
		FieldName:  field.Name,
		JSONName:   jsonName,
		Path:       path,
		FieldValue: valueAny,
	}

	err := fn(ctx, arg)
	if err == nil {
		return nil
	}
	if ve, ok := err.(*domain.ValidationError); ok {
		withResolvedMessage(ve, field, path, rule, arg, valueAny)
		ve.Rule = rule
		return ve
	}
	return withResolvedMessage(&domain.ValidationError{
		Field:   path,
		Message: err.Error(),
		Rule:    rule,
	}, field, path, rule, arg, valueAny)
}

func isZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return v.IsNil()
	default:
		return v.IsZero()
	}
}
