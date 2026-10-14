package validator

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"unicode"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// customValidatable is implemented by models with business rules after tags.
// Named differently from domain.Validator (engine interface).
type customValidatable interface {
	Validate() error
}

func runCustomValidate(v any) domain.ValidationErrors {
	if v == nil {
		return nil
	}
	if err := invokeCustomValidate(v); err != nil {
		return customValidateErrors(err)
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil
	}
	if rv.Kind() == reflect.Ptr {
		return nil
	}
	// Skip reflect.New when the type has no Validate() (common case).
	if !typeHasCustomValidate(rv.Type()) {
		return nil
	}
	if rv.CanAddr() {
		if err := invokeCustomValidate(rv.Addr().Interface()); err != nil {
			return customValidateErrors(err)
		}
		return nil
	}
	ptr := reflect.New(rv.Type())
	ptr.Elem().Set(rv)
	if err := invokeCustomValidate(ptr.Interface()); err != nil {
		return customValidateErrors(err)
	}
	return nil
}

var customValidateCache sync.Map // reflect.Type → bool

func typeHasCustomValidate(t reflect.Type) bool {
	if t == nil {
		return false
	}
	if v, ok := customValidateCache.Load(t); ok {
		return v.(bool)
	}
	has := false
	if t.Kind() != reflect.Ptr {
		pt := reflect.PointerTo(t)
		_, has = pt.MethodByName("Validate")
		if !has {
			_, has = t.MethodByName("Validate")
		}
	} else {
		_, has = t.MethodByName("Validate")
	}
	customValidateCache.Store(t, has)
	return has
}

func invokeCustomValidate(v any) error {
	if cv, ok := v.(customValidatable); ok {
		return cv.Validate()
	}
	return nil
}

func customValidateErrors(err error) domain.ValidationErrors {
	if err == nil {
		return nil
	}
	if ve, ok := err.(domain.ValidationErrors); ok {
		return ve
	}
	if ve, ok := err.(*domain.ValidationFailedError); ok {
		return ve.Errors
	}
	return domain.ValidationErrors{{
		Field:   "_",
		Message: err.Error(),
		Rule:    "custom",
	}}
}

func fieldByNameOrJSON(parent reflect.Value, name string) (reflect.Value, bool) {
	parent = derefValue(parent)
	if !parent.IsValid() || parent.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	if f := parent.FieldByName(name); f.IsValid() {
		return f, true
	}
	t := parent.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		if jsonFieldName(sf) == name {
			return parent.Field(i), true
		}
	}
	return reflect.Value{}, false
}

func fieldString(v reflect.Value) string {
	v = derefValue(v)
	if !v.IsValid() {
		return ""
	}
	return fmt.Sprintf("%v", v.Interface())
}

func splitFieldCond(arg string) (field, want string, ok bool) {
	arg = strings.TrimSpace(arg)
	parts := strings.SplitN(arg, " ", 2)
	if len(parts) < 2 || parts[0] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func validateRequiredIf(fieldName string, value, parent reflect.Value, arg string) *domain.ValidationError {
	otherName, want, ok := splitFieldCond(arg)
	if !ok {
		return nil
	}
	other, found := fieldByNameOrJSON(parent, otherName)
	if !found {
		return nil
	}
	if fieldString(other) != want {
		return nil
	}
	if isZero(value) {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: "is required",
			Rule:    "required_if",
		}
	}
	return nil
}

func validateRequiredUnless(fieldName string, value, parent reflect.Value, arg string) *domain.ValidationError {
	otherName, want, ok := splitFieldCond(arg)
	if !ok {
		return nil
	}
	other, found := fieldByNameOrJSON(parent, otherName)
	if !found {
		return nil
	}
	if fieldString(other) == want {
		return nil
	}
	if isZero(value) {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: "is required",
			Rule:    "required_unless",
		}
	}
	return nil
}

func validateRequiredWith(fieldName string, value, parent reflect.Value, arg string) *domain.ValidationError {
	names := strings.Fields(strings.TrimSpace(arg))
	if len(names) == 0 {
		return nil
	}
	anyPresent := false
	for _, name := range names {
		other, found := fieldByNameOrJSON(parent, name)
		if found && !isZero(other) {
			anyPresent = true
			break
		}
	}
	if !anyPresent {
		return nil
	}
	if isZero(value) {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: "is required",
			Rule:    "required_with",
		}
	}
	return nil
}

func validateEqField(fieldName string, value, parent reflect.Value, otherName string, equal bool) *domain.ValidationError {
	other, found := fieldByNameOrJSON(parent, strings.TrimSpace(otherName))
	if !found {
		return nil
	}
	same := fieldString(value) == fieldString(other)
	if equal && !same {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must be equal to %s", otherName),
			Rule:    "eqfield",
		}
	}
	if !equal && same {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must not be equal to %s", otherName),
			Rule:    "nefield",
		}
	}
	return nil
}

func isNumericKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func compareFieldValues(a, b reflect.Value) (int, bool) {
	a, b = derefValue(a), derefValue(b)
	if !a.IsValid() || !b.IsValid() {
		return 0, false
	}
	if isNumericKind(a.Kind()) && isNumericKind(b.Kind()) {
		fa, fb := numericToFloat(a), numericToFloat(b)
		switch {
		case fa < fb:
			return -1, true
		case fa > fb:
			return 1, true
		default:
			return 0, true
		}
	}
	return strings.Compare(fieldString(a), fieldString(b)), true
}

func validateCmpField(fieldName string, value, parent reflect.Value, otherName, rule string) *domain.ValidationError {
	other, found := fieldByNameOrJSON(parent, strings.TrimSpace(otherName))
	if !found {
		return nil
	}
	cmp, ok := compareFieldValues(value, other)
	if !ok {
		return nil
	}
	fail := false
	msg := ""
	switch rule {
	case "gt":
		fail = cmp <= 0
		msg = fmt.Sprintf("must be greater than %s", otherName)
	case "gte":
		fail = cmp < 0
		msg = fmt.Sprintf("must be greater than or equal to %s", otherName)
	case "lt":
		fail = cmp >= 0
		msg = fmt.Sprintf("must be less than %s", otherName)
	case "lte":
		fail = cmp > 0
		msg = fmt.Sprintf("must be less than or equal to %s", otherName)
	}
	if fail {
		return &domain.ValidationError{Field: fieldName, Message: msg, Rule: rule + "field"}
	}
	return nil
}

func validateUnique(fieldName string, value reflect.Value) *domain.ValidationError {
	value = derefValue(value)
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return nil
	}
	seen := make(map[string]struct{}, value.Len())
	for i := 0; i < value.Len(); i++ {
		key := fieldString(value.Index(i))
		if _, dup := seen[key]; dup {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: "must contain unique values",
				Rule:    "unique",
			}
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateIP(fieldName string, value reflect.Value, rule string) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	s := strings.TrimSpace(value.String())
	if s == "" {
		return nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: "must be a valid IP address",
			Rule:    rule,
		}
	}
	switch rule {
	case "ipv4":
		if ip.To4() == nil {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: "must be a valid IPv4 address",
				Rule:    rule,
			}
		}
	case "ipv6":
		if ip.To4() != nil || ip.To16() == nil {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: "must be a valid IPv6 address",
				Rule:    rule,
			}
		}
	}
	return nil
}

func validateCIDR(fieldName string, value reflect.Value) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	s := strings.TrimSpace(value.String())
	if s == "" {
		return nil
	}
	if _, _, err := net.ParseCIDR(s); err != nil {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: "must be a valid CIDR",
			Rule:    "cidr",
		}
	}
	return nil
}

func validateE164(fieldName string, value reflect.Value) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	s := strings.TrimSpace(value.String())
	if s == "" {
		return nil
	}
	if !strings.HasPrefix(s, "+") || len(s) < 2 || len(s) > 16 {
		return e164Error(fieldName)
	}
	for _, r := range s[1:] {
		if !unicode.IsDigit(r) {
			return e164Error(fieldName)
		}
	}
	return nil
}

func e164Error(fieldName string) *domain.ValidationError {
	return &domain.ValidationError{
		Field:   fieldName,
		Message: "must be a valid E.164 phone number",
		Rule:    "e164",
	}
}
