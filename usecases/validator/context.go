package validator

import (
	"reflect"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// NewFieldContext builds a FieldContext from reflect.
// parent must be the struct Value that owns the field (already deref).
func NewFieldContext(root any, parent reflect.Value, field reflect.StructField, fieldValue reflect.Value, path string) domain.FieldContext {
	var parentAny any
	if parent.IsValid() && parent.CanInterface() {
		parentAny = parent.Interface()
	}
	var valueAny any
	if fieldValue.IsValid() && fieldValue.CanInterface() {
		valueAny = fieldValue.Interface()
	}
	return domain.FieldContext{
		Root:       root,
		Parent:     parentAny,
		FieldName:  field.Name,
		JSONName:   jsonFieldName(field),
		Path:       path,
		FieldValue: valueAny,
	}
}

// FieldContextFromValues is a helper without StructField (tests / manual calls).
func FieldContextFromValues(root, parent any, fieldName, jsonName, path string, value any) domain.FieldContext {
	if jsonName == "" {
		jsonName = fieldName
	}
	if path == "" {
		path = jsonName
	}
	if parent == nil {
		parent = root
	}
	return domain.FieldContext{
		Root:       root,
		Parent:     parent,
		FieldName:  fieldName,
		JSONName:   jsonName,
		Path:       path,
		FieldValue: value,
	}
}

// WrapLegacyRule adapts the old applyRule(fieldName, value, parent, rule, arg)
// signature to ValidatorFunc — used in the gradual builtin migration (phase 2).
func WrapLegacyRule(rule string, apply func(fieldName string, value, parent reflect.Value, arg string) *domain.ValidationError) domain.ValidatorFunc {
	return func(ctx domain.FieldContext, param string) error {
		parent := reflect.ValueOf(ctx.Parent)
		value := reflect.ValueOf(ctx.FieldValue)
		path := ctx.Path
		if path == "" {
			path = ctx.JSONName
		}
		if err := apply(path, value, parent, param); err != nil {
			return err
		}
		return nil
	}
}
