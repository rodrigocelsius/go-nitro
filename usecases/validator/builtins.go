package validator

import (
	"reflect"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// registerBuiltins registers all native rules on the registry.
// required / omitempty / dive stay as parser meta (not ValidatorFunc).
// Locale document rules (cpf, cnpj, phone_br, cep, uf) stay in package br — not here.
func registerBuiltins(r *Registry) {
	if r == nil {
		return
	}

	// --- conditional presence (no early deref) ---
	r.Register("required_if", legacyNoDeref(validateRequiredIf))
	r.Register("required_unless", legacyNoDeref(validateRequiredUnless))
	r.Register("required_with", legacyNoDeref(validateRequiredWith))

	// --- size / set / comparison ---
	r.Register("min", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateMin(path, value, arg)
	}))
	r.Register("max", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateMax(path, value, arg)
	}))
	r.Register("len", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateLen(path, value, arg)
	}))
	r.Register("in", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateIn(path, value, arg)
	}))
	r.Register("not_in", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateNotIn(path, value, arg)
	}))
	r.Register("email", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateEmail(path, value)
	}))
	r.Register("oneof", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateOneOf(path, value, arg)
	}))
	r.Register("eq", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateEq(path, value, arg)
	}))
	r.Register("ne", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateNe(path, value, arg)
	}))
	r.Register("gt", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateGt(path, value, arg)
	}))
	r.Register("gte", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateGte(path, value, arg)
	}))
	r.Register("lt", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateLt(path, value, arg)
	}))
	r.Register("lte", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateLte(path, value, arg)
	}))

	// --- identity / string / time ---
	r.Register("uuid", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateUUID(path, value, "uuid")
	}))
	r.Register("uuid4", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateUUID(path, value, "uuid4")
	}))
	r.Register("uuid7", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateUUID(path, value, "uuid7")
	}))
	r.Register("url", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateURL(path, value)
	}))
	r.Register("uri", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateURI(path, value)
	}))
	r.Register("alpha", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateAlpha(path, value)
	}))
	r.Register("alphanum", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateAlphanum(path, value)
	}))
	r.Register("numeric", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateNumeric(path, value)
	}))
	r.Register("startswith", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateStartsWith(path, value, arg)
	}))
	r.Register("endswith", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateEndsWith(path, value, arg)
	}))
	r.Register("contains", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateContains(path, value, arg)
	}))
	r.Register("excludes", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateExcludes(path, value, arg)
	}))
	r.Register("datetime", legacyDeref(func(path string, value, _ reflect.Value, arg string) *domain.ValidationError {
		return validateDatetime(path, value, arg)
	}))

	// --- cross-field ---
	r.Register("eqfield", legacyDeref(func(path string, value, parent reflect.Value, arg string) *domain.ValidationError {
		return validateEqField(path, value, parent, arg, true)
	}))
	r.Register("nefield", legacyDeref(func(path string, value, parent reflect.Value, arg string) *domain.ValidationError {
		return validateEqField(path, value, parent, arg, false)
	}))
	r.Register("gtfield", legacyDeref(func(path string, value, parent reflect.Value, arg string) *domain.ValidationError {
		return validateCmpField(path, value, parent, arg, "gt")
	}))
	r.Register("gtefield", legacyDeref(func(path string, value, parent reflect.Value, arg string) *domain.ValidationError {
		return validateCmpField(path, value, parent, arg, "gte")
	}))
	r.Register("ltfield", legacyDeref(func(path string, value, parent reflect.Value, arg string) *domain.ValidationError {
		return validateCmpField(path, value, parent, arg, "lt")
	}))
	r.Register("ltefield", legacyDeref(func(path string, value, parent reflect.Value, arg string) *domain.ValidationError {
		return validateCmpField(path, value, parent, arg, "lte")
	}))

	// --- collection / network ---
	r.Register("unique", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateUnique(path, value)
	}))
	r.Register("ip", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateIP(path, value, "ip")
	}))
	r.Register("ipv4", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateIP(path, value, "ipv4")
	}))
	r.Register("ipv6", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateIP(path, value, "ipv6")
	}))
	r.Register("cidr", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateCIDR(path, value)
	}))
	r.Register("e164", legacyDeref(func(path string, value, _ reflect.Value, _ string) *domain.ValidationError {
		return validateE164(path, value)
	}))

	// Aliases (validation.md / Laravel-style)
	r.RegisterAlias("same", "eqfield")
	r.RegisterAlias("eq_field", "eqfield")
}

type legacyFn func(path string, value, parent reflect.Value, arg string) *domain.ValidationError

// legacyDeref mirrors the old applyRule: deref and skip if invalid.
func legacyDeref(fn legacyFn) domain.ValidatorFunc {
	return func(ctx domain.FieldContext, param string) error {
		value := derefValue(reflectValueOf(ctx.FieldValue))
		if !value.IsValid() {
			return nil
		}
		parent := reflectValueOf(ctx.Parent)
		path := ctx.Path
		if path == "" {
			path = ctx.JSONName
		}
		if err := fn(path, value, parent, param); err != nil {
			return err
		}
		return nil
	}
}

// legacyNoDeref is used for required_if/unless/with (inspect zero on the original Value).
func legacyNoDeref(fn legacyFn) domain.ValidatorFunc {
	return func(ctx domain.FieldContext, param string) error {
		value := reflectValueOf(ctx.FieldValue)
		parent := reflectValueOf(ctx.Parent)
		path := ctx.Path
		if path == "" {
			path = ctx.JSONName
		}
		if err := fn(path, value, parent, param); err != nil {
			return err
		}
		return nil
	}
}

func reflectValueOf(v any) reflect.Value {
	if v == nil {
		return reflect.Value{}
	}
	return reflect.ValueOf(v)
}

func init() {
	registerBuiltins(defaultRegistry)
	defaultRegistry.StrictUnknown = true
}
