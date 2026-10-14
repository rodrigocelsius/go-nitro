package validator

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

func validateUUID(fieldName string, value reflect.Value, rule string) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	u, err := nitrouuid.Parse(value.String())
	if err != nil {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: "must be a valid UUID",
			Rule:    rule,
		}
	}
	switch rule {
	case "uuid4":
		if u.Version() != 4 {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: "must be a UUID version 4",
				Rule:    rule,
			}
		}
	case "uuid7":
		if u.Version() != 7 {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: "must be a UUID version 7",
				Rule:    rule,
			}
		}
	}
	return nil
}

func validateURL(fieldName string, value reflect.Value) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	s := strings.TrimSpace(value.String())
	u, err := url.ParseRequestURI(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: "must be a valid URL",
			Rule:    "url",
		}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: "must be a valid URL",
			Rule:    "url",
		}
	}
	return nil
}

func validateURI(fieldName string, value reflect.Value) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	s := strings.TrimSpace(value.String())
	u, err := url.ParseRequestURI(s)
	if err != nil || u.Scheme == "" {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: "must be a valid URI",
			Rule:    "uri",
		}
	}
	return nil
}

func validateAlpha(fieldName string, value reflect.Value) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	s := value.String()
	if s == "" {
		return nil
	}
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: "must contain only letters",
				Rule:    "alpha",
			}
		}
	}
	return nil
}

func validateAlphanum(fieldName string, value reflect.Value) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	s := value.String()
	if s == "" {
		return nil
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return &domain.ValidationError{
				Field:   fieldName,
				Message: "must contain only letters and numbers",
				Rule:    "alphanum",
			}
		}
	}
	return nil
}

func validateNumeric(fieldName string, value reflect.Value) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	s := value.String()
	if s == "" {
		return nil
	}
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: "must be a numeric value",
			Rule:    "numeric",
		}
	}
	return nil
}

func validateStartsWith(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	if !strings.HasPrefix(value.String(), arg) {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must start with %q", arg),
			Rule:    "startswith",
		}
	}
	return nil
}

func validateEndsWith(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	if !strings.HasSuffix(value.String(), arg) {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must end with %q", arg),
			Rule:    "endswith",
		}
	}
	return nil
}

func validateContains(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	if !strings.Contains(value.String(), arg) {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must contain %q", arg),
			Rule:    "contains",
		}
	}
	return nil
}

func validateExcludes(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	if strings.Contains(value.String(), arg) {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must not contain %q", arg),
			Rule:    "excludes",
		}
	}
	return nil
}

func validateDatetime(fieldName string, value reflect.Value, arg string) *domain.ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}
	layout := arg
	if layout == "" {
		layout = time.RFC3339
	}
	if _, err := time.Parse(layout, value.String()); err != nil {
		return &domain.ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must be a valid datetime (%s)", layout),
			Rule:    "datetime",
		}
	}
	return nil
}
