package nitro

import (
	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator/br"
)

// ValidatorFunc is the signature for custom validation rules.
type ValidatorFunc = domain.ValidatorFunc

// FieldContext is passed to ValidatorFunc.
type FieldContext = domain.FieldContext

// Validator validates structs (tags + registered rules).
type Validator = domain.Validator

// RegistrableValidator is a Validator that accepts Register/RegisterAlias.
type RegistrableValidator = domain.RegistrableValidator

// Rule is a parsed validate tag rule.
type Rule = usecases.Rule

// Options configures StructValidator (StopOnFirstError, StrictUnknown).
type Options = usecases.Options

// NewValidator creates an isolated validator with all builtins (StrictUnknown=true).
// Prefer per-app instances for locale plugins (RegisterBRValidators) and app rules.
func NewValidator() *usecases.StructValidator {
	return usecases.NewValidator()
}

// DefaultValidator returns the process-wide validator (builtins + RegisterValidator).
func DefaultValidator() *usecases.StructValidator {
	return usecases.DefaultValidator()
}

// RegisterValidator registers a rule on the process-wide default validator.
// Prefer app.SetValidator(NewValidator()) + Register for app-scoped rules.
func RegisterValidator(name string, fn ValidatorFunc) {
	usecases.RegisterValidator(name, fn)
}

// RegisterAlias registers an alias on the default validator (e.g. same → eqfield).
// Prefer NewValidator().RegisterAlias for app-scoped aliases.
func RegisterAlias(alias, target string) {
	usecases.RegisterValidatorAlias(alias, target)
}

// ParseRules parses a validate tag into []Rule.
func ParseRules(tag string) []Rule {
	return usecases.ParseRules(tag)
}

// Validate runs validation with the default validator.
func Validate(v any) domain.ValidationErrors {
	return usecases.ValidateStruct(v)
}

// ApplyTransforms applies `transform:"…"` tags on string / *string
// (titlecase, digits, upper, trim, camelcase, …). BodyJSON/Parse already
// call this after defaults and before validation.
func ApplyTransforms(v any) {
	usecases.ApplyTransforms(v)
}

// UnmarshalJSONPatch merges JSON onto an existing value and applies transforms (no defaults).
// Typical use: public PATCH after FindByID.
//
//	if err := nitro.UnmarshalJSONPatch(c.Body(), &existing); err != nil { ... }
func UnmarshalJSONPatch(data []byte, v any) error {
	return validator.UnmarshalJSONPatch(data, v)
}

// ValidationErrors is the list type returned by Validate.
type ValidationErrors = domain.ValidationErrors

// NewValidationFailed wraps ValidationErrors as HTTP 422.
func NewValidationFailed(errs ValidationErrors) error {
	return domain.NewValidationFailed(errs)
}

// MustValidate runs v.Validate(entity) and returns HTTP 422 on failure.
// If v is nil, uses DefaultValidator().
//
// Use after UnmarshalJSONPatch (transforms already applied). Skip after Parse —
// Parse already validates.
func MustValidate(v Validator, entity any) error {
	if v == nil {
		v = DefaultValidator()
	}
	errs := v.Validate(entity)
	if !errs.HasErrors() {
		return nil
	}
	return NewValidationFailed(errs)
}

// TitleCase capitalizes words; use TitleCaseOptions.Exceptions for PT stopwords.
func TitleCase(s string, opts transform.TitleCaseOptions) string {
	return transform.TitleCase(s, opts)
}

// TitleCaseOptions configures TitleCase exceptions.
type TitleCaseOptions = transform.TitleCaseOptions

// DefaultTitleCasePT returns PT-BR options (de/da/do/…).
func DefaultTitleCasePT() TitleCaseOptions {
	return transform.DefaultTitleCasePT()
}

// RegisterBRValidators registers cpf, cnpj, phone_br, cep, uf on the validator.
// Opt-in — implemented in usecases/validator/br; not part of NewValidator().
// App rules (SKU, …): v.Register in the app. PT transforms: transform:"titlecase=pt".
//
//	v := nitro.NewValidator()
//	nitro.RegisterBRValidators(v)
//	app := nitro.New(nitro.Config{Validator: v})
func RegisterBRValidators(v RegistrableValidator) {
	br.Register(v)
}

// SetValidator sets the app validator used by BodyJSON / Parse / Resource.
func (a *App) SetValidator(v Validator) *App {
	a.inner.SetValidator(v)
	a.cfg.Validator = v
	return a
}

// Validator returns the app validator.
func (a *App) Validator() Validator {
	return a.inner.Validator()
}

// PatchParse reads the request body, UnmarshalJSONPatch onto existing, then MustValidate.
//
//	if err := nitro.PatchParse(c, existing); err != nil {
//	    return err
//	}
func PatchParse(c *Ctx, existing any) error {
	if c == nil || existing == nil {
		return BadRequest("invalid patch request")
	}
	if err := UnmarshalJSONPatch(c.Body(), existing); err != nil {
		return BadRequest("invalid json body")
	}
	return MustValidate(nil, existing)
}
