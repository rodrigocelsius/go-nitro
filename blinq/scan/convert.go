package scan

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// AsString converts common DB/JSON values to string.
func AsString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	case fmt.Stringer:
		return s.String()
	default:
		return fmt.Sprint(s)
	}
}

// AsBool converts common values to bool.
func AsBool(v any) bool {
	if v == nil {
		return false
	}
	switch b := v.(type) {
	case bool:
		return b
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(b).Int() != 0
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(b).Uint() != 0
	case float32, float64:
		return reflect.ValueOf(b).Float() != 0
	case string:
		ok, _ := strconv.ParseBool(b)
		return ok
	case []byte:
		ok, _ := strconv.ParseBool(string(b))
		return ok
	default:
		return false
	}
}

// AsInt64 converts common values to int64.
func AsInt64(v any) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int16:
		return int64(n)
	case int8:
		return int64(n)
	case uint64:
		return int64(n)
	case uint:
		return int64(n)
	case uint32:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	case []byte:
		i, _ := strconv.ParseInt(string(n), 10, 64)
		return i
	default:
		return asInt64Reflect(v)
	}
}

// AsInt converts to int.
func AsInt(v any) int { return int(AsInt64(v)) }

// AsFloat64 converts common values to float64.
// Driver-specific types (e.g. pgtype.Numeric) can be
// handled via AsFloat64Hook.
func AsFloat64(v any) float64 {
	if v == nil {
		return 0
	}
	if AsFloat64Hook != nil {
		if f, ok := AsFloat64Hook(v); ok {
			return f
		}
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case uint64:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	case []byte:
		f, _ := strconv.ParseFloat(string(n), 64)
		return f
	default:
		return asFloat64Reflect(v)
	}
}

// AsFloat64Hook lets adapters (pgx) register extra types without
// coupling the runtime to drivers.
var AsFloat64Hook func(v any) (float64, bool)

// AsTime converts to time.Time.
func AsTime(v any) time.Time {
	if v == nil {
		return time.Time{}
	}
	switch t := v.(type) {
	case time.Time:
		return t
	case *time.Time:
		if t == nil {
			return time.Time{}
		}
		return *t
	case string:
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

// Row is an ergonomic shortcut over map[string]any from the scanner.
type Row map[string]any

// Rows converts []map to []Row without deep-copying entries.
func Rows(maps []map[string]any) []Row {
	out := make([]Row, len(maps))
	for i, m := range maps {
		out[i] = Row(m)
	}
	return out
}

func (r Row) Get(key string) any {
	if r == nil {
		return nil
	}
	return r[key]
}

func (r Row) String(key string) string   { return AsString(r.Get(key)) }
func (r Row) Bool(key string) bool       { return AsBool(r.Get(key)) }
func (r Row) Int64(key string) int64     { return AsInt64(r.Get(key)) }
func (r Row) Int(key string) int         { return AsInt(r.Get(key)) }
func (r Row) Float64(key string) float64 { return AsFloat64(r.Get(key)) }
func (r Row) Time(key string) time.Time  { return AsTime(r.Get(key)) }

func (r Row) Has(key string) bool {
	if r == nil {
		return false
	}
	_, ok := r[key]
	return ok
}

func asInt64Reflect(v any) int64 {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(rv.Uint())
	case reflect.Float32, reflect.Float64:
		return int64(rv.Float())
	}
	return 0
}

func asFloat64Reflect(v any) float64 {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Float32, reflect.Float64:
		return rv.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint())
	}
	return 0
}
