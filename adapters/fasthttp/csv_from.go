package fasthttp

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// JSONCell marshals v as a single CSV cell (JSON text). Nil / empty slice → "[]".
func JSONCell(v any) string {
	if v == nil {
		return "[]"
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		if rv.IsNil() {
			return "[]"
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// CSVFrom exports a slice of structs as CSV using json tag names as columns.
// Non-scalar fields (slice/map/struct) are written via JSONCell.
//
//	return c.CSVFrom("drivers.csv", drivers, "name", "phone", "missions")
func (c *FastCtx) CSVFrom(filename string, rows any, columns ...string) error {
	header, data, err := csvFromReflect(rows, columns...)
	if err != nil {
		return err
	}
	return c.CSV(filename, header, data)
}

func csvFromReflect(rows any, columns ...string) ([]string, [][]string, error) {
	if len(columns) == 0 {
		return nil, nil, fmt.Errorf("fasthttp: CSVFrom requires at least one column")
	}
	rv := reflect.ValueOf(rows)
	if !rv.IsValid() {
		return columns, nil, nil
	}
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return columns, nil, nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return nil, nil, fmt.Errorf("fasthttp: CSVFrom expects a slice, got %s", rv.Kind())
	}
	out := make([][]string, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		item := rv.Index(i)
		for item.Kind() == reflect.Interface || item.Kind() == reflect.Pointer {
			if item.IsNil() {
				break
			}
			item = item.Elem()
		}
		if !item.IsValid() || item.Kind() != reflect.Struct {
			return nil, nil, fmt.Errorf("fasthttp: CSVFrom row %d is not a struct", i)
		}
		row := make([]string, len(columns))
		for j, col := range columns {
			row[j] = csvFieldValue(item, col)
		}
		out = append(out, row)
	}
	return columns, out, nil
}

func csvFieldValue(structVal reflect.Value, jsonName string) string {
	if matched, v := csvLookup(structVal, jsonName); matched {
		return v
	}
	return ""
}

func csvLookup(structVal reflect.Value, jsonName string) (bool, string) {
	t := structVal.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			fv := structVal.Field(i)
			for fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					return false, ""
				}
				fv = fv.Elem()
			}
			if fv.IsValid() && fv.Kind() == reflect.Struct {
				if ok, v := csvLookup(fv, jsonName); ok {
					return true, v
				}
			}
			continue
		}
		if fieldJSONName(f) == jsonName {
			return true, csvScalarOrJSON(structVal.Field(i))
		}
	}
	return false, ""
}

func fieldJSONName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" || tag == "-" {
		return ""
	}
	return strings.Split(tag, ",")[0]
}

func csvScalarOrJSON(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Slice, reflect.Map, reflect.Struct, reflect.Array:
		return JSONCell(v.Interface())
	default:
		return fmt.Sprint(v.Interface())
	}
}
