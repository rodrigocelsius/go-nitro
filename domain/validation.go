package domain

import (
	"fmt"
	"strings"
)

// ValidationError represents a field validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Rule    string `json:"rule"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a list of validation errors.
type ValidationErrors []*ValidationError

func (e ValidationErrors) Error() string {
	msgs := make([]string, len(e))
	for i, err := range e {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "; ")
}

// HasErrors reports whether there are validation errors.
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Map returns a field→message map of validation errors.
// Useful for structured JSON responses.
//
// Example:
//
//	{"email": "is required", "password": "must be at least 6 characters"}
func (e ValidationErrors) Map() map[string]string {
	result := make(map[string]string, len(e))
	for _, err := range e {
		result[err.Field] = err.Message
	}
	return result
}

// FieldContext provides field context to ValidatorFunc.
// No reflect in domain: only any / string values.
type FieldContext struct {
	// Root is the root struct being validated (or a pointer).
	Root any
	// Parent is the immediate parent struct (nested); for top-level fields = Root.
	Parent any
	// FieldName is the Go field name.
	FieldName string
	// JSONName is the field's json name (no path).
	JSONName string
	// Path is the full path (e.g. "address.street", "tags[0]").
	Path string
	// FieldValue is the current field value.
	FieldValue any
}

// ValidatorFunc is the signature of a rule registered in the registry.
// param comes from the tag (e.g. min=3 → "3"; required_if=Status paid → "Status paid").
type ValidatorFunc func(ctx FieldContext, param string) error

// Validator validates structs (tags + registered rules).
type Validator interface {
	Validate(v any) ValidationErrors
}

// RuleRegistry allows registering custom rules (e.g. cpf).
// Implemented by the engine in usecases; apps register domain rules here.
type RuleRegistry interface {
	Register(name string, fn ValidatorFunc)
	RegisterAlias(alias, target string)
	Lookup(name string) (ValidatorFunc, bool)
}

// RegistrableValidator combines validation and rule registration.
type RegistrableValidator interface {
	Validator
	RuleRegistry
}

// ValidationFailedError is an HTTP 422 error with structured validation fields.
//
// JSON response: {"error": "validation failed", "fields": {"email": "is required", ...}}
type ValidationFailedError struct {
	Errors ValidationErrors
}

func (e *ValidationFailedError) Error() string   { return "validation failed" }
func (e *ValidationFailedError) StatusCode() int { return 422 }
func (e *ValidationFailedError) Body() any {
	return map[string]any{
		"error":  "validation failed",
		"fields": e.Errors.Map(),
	}
}

// NewValidationFailed creates a ValidationFailedError from ValidationErrors.
func NewValidationFailed(errs ValidationErrors) error {
	return &ValidationFailedError{Errors: errs}
}
