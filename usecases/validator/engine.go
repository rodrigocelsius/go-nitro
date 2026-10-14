// Package validator implements the registry-driven struct validation engine.
//
// Layout (Clean Arch / validation.md):
//
//	parser.go          — ParseRules / parseValidateTag
//	registry.go        — Registry, aliases, default singleton
//	engine.go          — StructValidator
//	options.go         — Options (StopOnFirstError, StrictUnknown)
//	cache.go           — type meta sync.Map
//	validate.go        — validateValue / applyRule
//	messages.go        — msg_<rule> / message / placeholders
//	context.go         — FieldContext helpers
//	builtins.go        — registerBuiltins (+ aliases same/eq_field)
//	builtin_rules.go   — min/max/len/email/…
//	builtin_cross.go   — required_if, eqfield, …
//	builtin_extra.go   — uuid, url, alpha, …
//	br/                — locale document rules (cpf, cnpj, …); Register only, never builtins
//
// Layers:
//   - builtins: NewValidator() (email, uuid, min, e164, …)
//   - locale docs: usecases/validator/br via nitro.RegisterBRValidators (opt-in)
//   - app domain: v.Register("sku", …) in the application, not this package
package validator

import (
	"reflect"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// StructValidator is the registry-driven validation engine.
// It implements domain.RegistrableValidator.
type StructValidator struct {
	*Registry
	Options Options
}

// NewValidator creates a validator with all builtins registered.
func NewValidator() *StructValidator {
	r := NewRegistry()
	registerBuiltins(r)
	return &StructValidator{
		Registry: r,
		Options:  DefaultOptions(),
	}
}

// DefaultValidator returns the process-wide validator (builtins + RegisterValidator).
func DefaultValidator() *StructValidator {
	return &StructValidator{
		Registry: DefaultRegistry(),
		Options:  DefaultOptions(),
	}
}

// effectiveOptions merges Options.StopOnFirstError with Registry.StrictUnknown
// (legacy API: v.StrictUnknown = false).
func (v *StructValidator) effectiveOptions() Options {
	opts := DefaultOptions()
	if v == nil {
		return opts
	}
	opts.StopOnFirstError = v.Options.StopOnFirstError
	if v.Registry != nil {
		opts.StrictUnknown = v.Registry.StrictUnknown
	} else {
		opts.StrictUnknown = v.Options.StrictUnknown
	}
	return opts
}

// SetOptions applies Options and syncs StrictUnknown on the Registry.
func (v *StructValidator) SetOptions(o Options) *StructValidator {
	if v == nil {
		return v
	}
	v.Options = o
	if v.Registry != nil {
		v.Registry.StrictUnknown = o.StrictUnknown
	}
	return v
}

// Validate runs validate tags plus a custom Validate() on the model.
func (v *StructValidator) Validate(data any) domain.ValidationErrors {
	reg := DefaultRegistry()
	if v != nil && v.Registry != nil {
		reg = v.Registry
	}
	opts := v.effectiveOptions()
	errs := validateValue("", reflect.ValueOf(data), data, reg, opts)
	if opts.StopOnFirstError && len(errs) > 0 {
		return errs
	}
	errs = append(errs, runCustomValidate(data)...)
	return errs
}

// Ensure interface compliance.
var (
	_ domain.Validator            = (*StructValidator)(nil)
	_ domain.RuleRegistry         = (*StructValidator)(nil)
	_ domain.RegistrableValidator = (*StructValidator)(nil)
)
