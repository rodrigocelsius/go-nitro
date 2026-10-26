package usecases

import (
	"reflect"
	"strconv"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// BindParams extracts and converts path parameters into the destination struct.
func BindParams(c *domain.Ctx, dest any) error {
	return bindFromSource(c, dest, domain.SourcePath)
}

// BindQuery extracts and converts query parameters into the destination struct.
func BindQuery(c *domain.Ctx, dest any) error {
	return bindFromSource(c, dest, domain.SourceQuery)
}

func bindFromSource(c *domain.Ctx, dest any, source domain.ParamSource) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return &domain.BindError{Source: source, Message: "destination must be a pointer to struct"}
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		name := field.Tag.Get("param")
		if name == "" {
			name = field.Tag.Get("query")
		}
		if name == "" {
			continue
		}

		var raw string
		switch source {
		case domain.SourcePath:
			raw = c.Param(name)
		case domain.SourceQuery:
			raw = c.QueryParam(name)
		case domain.SourceHeader:
			raw = c.HeaderParam(name)
		}

		if raw == "" {
			continue
		}

		if err := convertAndSet(fieldValue, raw); err != nil {
			return &domain.BindError{
				Field:   field.Name,
				Source:  source,
				Message: err.Error(),
			}
		}
	}

	return nil
}

func convertAndSet(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	default:
		return &domain.BindError{Message: "unsupported type: " + field.Type().String()}
	}
	return nil
}
