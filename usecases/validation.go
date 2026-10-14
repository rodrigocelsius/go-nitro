package usecases

import (
	"reflect"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

// Validation facade — implementation in usecases/validator (Clean Arch).

// StructValidator is an alias of the registry-driven validator engine.
type StructValidator = validator.StructValidator

// Registry is an alias of the rule registry.
type Registry = validator.Registry

// Rule is a parsed validate tag rule.
type Rule = validator.Rule

// Options configures StructValidator (StopOnFirstError, StrictUnknown).
type Options = validator.Options

// DefaultOptions returns Options with StrictUnknown=true.
func DefaultOptions() Options {
	return validator.DefaultOptions()
}

// NewValidator creates an isolated validator with all builtins.
func NewValidator() *validator.StructValidator {
	return validator.NewValidator()
}

// DefaultValidator returns the process-wide validator.
func DefaultValidator() *validator.StructValidator {
	return validator.DefaultValidator()
}

// DefaultRegistry returns the process-wide rule registry.
func DefaultRegistry() *validator.Registry {
	return validator.DefaultRegistry()
}

// SetDefaultRegistry replaces the process-wide registry (tests / advanced boot).
func SetDefaultRegistry(r *validator.Registry) {
	validator.SetDefaultRegistry(r)
}

// NewRegistry creates an empty registry (StrictUnknown=true).
func NewRegistry() *validator.Registry {
	return validator.NewRegistry()
}

// ValidateStruct validates a struct using validate tags (default validator).
func ValidateStruct(v any) domain.ValidationErrors {
	return validator.ValidateStruct(v)
}

// ApplyTransforms applies `transform:` tags (default → transform → validate pipeline).
func ApplyTransforms(v any) {
	validator.ApplyTransforms(v)
}

// ParseRules parses a validate tag into []Rule.
func ParseRules(tag string) []Rule {
	return validator.ParseRules(tag)
}

// RegisterValidator registers a rule on the default registry.
func RegisterValidator(name string, fn domain.ValidatorFunc) {
	validator.RegisterValidator(name, fn)
}

// RegisterValidatorAlias registers an alias on the default registry.
func RegisterValidatorAlias(alias, target string) {
	validator.RegisterValidatorAlias(alias, target)
}

// FieldContextFromValues builds a FieldContext without reflect.StructField.
func FieldContextFromValues(root, parent any, fieldName, jsonName, path string, value any) domain.FieldContext {
	return validator.FieldContextFromValues(root, parent, fieldName, jsonName, path, value)
}

// ExpandMessage expands {field}/{param}/{value}/{rule} placeholders.
func ExpandMessage(tmpl, field, rule, param string, value any) string {
	return validator.ExpandMessage(tmpl, field, rule, param, value)
}

// WrapLegacyRule adapts the old applyRule signature to ValidatorFunc.
func WrapLegacyRule(rule string, apply func(fieldName string, value, parent reflect.Value, arg string) *domain.ValidationError) domain.ValidatorFunc {
	return validator.WrapLegacyRule(rule, apply)
}
