package validator

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func validateMin(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	min, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return nil
	}

	switch value.Kind() {
	case reflect.String:
		if int64(len(value.String())) < min {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must be at least %d characters", min),
				Rule:    "min",
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value.Int() < min {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must be at least %d", min),
				Rule:    "min",
			}
		}
	case reflect.Slice, reflect.Map, reflect.Array:
		if int64(value.Len()) < min {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must have at least %d items", min),
				Rule:    "min",
			}
		}
	}
	return nil
}

func validateMax(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	max, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return nil
	}

	switch value.Kind() {
	case reflect.String:
		if int64(len(value.String())) > max {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must be at most %d characters", max),
				Rule:    "max",
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value.Int() > max {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must be at most %d", max),
				Rule:    "max",
			}
		}
	case reflect.Slice, reflect.Map, reflect.Array:
		if int64(value.Len()) > max {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must have at most %d items", max),
				Rule:    "max",
			}
		}
	}
	return nil
}

func validateLen(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	length, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return nil
	}

	switch value.Kind() {
	case reflect.String:
		if int64(len(value.String())) != length {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must be exactly %d characters", length),
				Rule:    "len",
			}
		}
	case reflect.Slice, reflect.Map, reflect.Array:
		if int64(value.Len()) != length {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("must have exactly %d items", length),
				Rule:    "len",
			}
		}
	}
	return nil
}

func validateIn(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	strValue := fieldStringFast(value)
	if inSpaceSeparated(arg, strValue) {
		return nil
	}
	return &domain.ValidationError{
		Field:   fieldName,
		Message: fmt.Sprintf("must be one of: %s", arg),
		Rule:    "in",
	}
}

func validateNotIn(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	strValue := fieldStringFast(value)
	if inSpaceSeparated(arg, strValue) {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must not be one of: %s", arg),
			Rule:    "not_in",
		}
	}
	return nil
}

// inSpaceSeparated compares without strings.Split (zero alloc on the match path).
func inSpaceSeparated(arg, want string) bool {
	start := 0
	for i := 0; i <= len(arg); i++ {
		if i == len(arg) || arg[i] == ' ' {
			if i > start && arg[start:i] == want {
				return true
			}
			start = i + 1
		}
	}
	return false
}

func fieldStringFast(value reflect.Value) string {
	if value.Kind() == reflect.String {
		return value.String()
	}
	if value.IsValid() && value.CanInterface() {
		return fmt.Sprintf("%v", value.Interface())
	}
	return ""
}

func validateEmail(fieldName string, value reflect.Value) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	if !IsEmail(value.String()) {
		return emailValidationError(fieldName)
	}
	return nil
}

func emailValidationError(fieldName string) *domain.ValidationError {
	return &domain.ValidationError{
		Field:   fieldName,
		Message: "must be a valid email",
		Rule:    "email",
	}
}

func validateOneOf(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	strValue := fieldStringFast(value)
	if inSpaceSeparated(arg, strValue) {
		return nil
	}
	return &domain.ValidationError{
		Field:   fieldName,
		Message: fmt.Sprintf("must be one of: %s", arg),
		Rule:    "oneof",
	}
}

func validateEq(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	strValue := fmt.Sprintf("%v", value.Interface())
	if strValue != arg {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must be equal to %s", arg),
			Rule:    "eq",
		}
	}
	return nil
}

func validateNe(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	strValue := fmt.Sprintf("%v", value.Interface())
	if strValue == arg {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must not be equal to %s", arg),
			Rule:    "ne",
		}
	}
	return nil
}

// parseNumericArg converts the argument to a float64 for numeric comparisons.
func parseNumericArg(arg string) (float64, error) {
	return strconv.ParseFloat(arg, 64)
}

func numericToFloat(v reflect.Value) float64 {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		return v.Float()
	}
	return 0
}

func validateGt(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	threshold, err := parseNumericArg(arg)
	if err != nil {
		return nil
	}
	val := numericToFloat(value)
	if val <= threshold {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must be greater than %s", arg),
			Rule:    "gt",
		}
	}
	return nil
}

func validateGte(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	threshold, err := parseNumericArg(arg)
	if err != nil {
		return nil
	}
	val := numericToFloat(value)
	if val < threshold {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must be greater than or equal to %s", arg),
			Rule:    "gte",
		}
	}
	return nil
}

func validateLt(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	threshold, err := parseNumericArg(arg)
	if err != nil {
		return nil
	}
	val := numericToFloat(value)
	if val >= threshold {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must be less than %s", arg),
			Rule:    "lt",
		}
	}
	return nil
}

func validateLte(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	threshold, err := parseNumericArg(arg)
	if err != nil {
		return nil
	}
	val := numericToFloat(value)
	if val > threshold {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must be less than or equal to %s", arg),
			Rule:    "lte",
		}
	}
	return nil
}
