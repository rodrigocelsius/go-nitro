package scan

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// MapToStruct fills dest from a column→value map.
// Uses `db` tags (or snake_case of the field name) and the As* converters.
//
// Column bindings are resolved once per struct type and memoized, so scanning
// many rows pays reflection/tag-parsing only on the first row.
func MapToStruct(row map[string]any, dest any) error {
	if dest == nil {
		return fmt.Errorf("blinq/scan: MapToStruct dest is nil")
	}
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("blinq/scan: MapToStruct dest must be non-nil *struct")
	}
	return mapToStructValue(row, v.Elem())
}

// MapsToSlice converts []map into []T.
func MapsToSlice[T any](rows []map[string]any) ([]T, error) {
	out := make([]T, 0, len(rows))
	var b *bindings
	for _, row := range rows {
		var item T
		if b == nil {
			b = bindingsOf(reflect.TypeOf(item))
		}
		if err := b.applyMap(row, reflect.ValueOf(&item).Elem()); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// fieldBinding maps one struct field (by index path, flattened through
// anonymous struct fields) to its resolved database column name.
type fieldBinding struct {
	path   []int
	column string
	name   string
}

// bindings is the memoized per-type scan plan.
type bindings struct {
	fields []fieldBinding
	byCol  map[string]int // column -> index into fields (-1 = ambiguous/duplicate)
}

var bindingCache sync.Map // reflect.Type -> *bindings

func bindingsOf(t reflect.Type) *bindings {
	if t == nil {
		t = reflect.TypeOf(struct{}{})
	}
	if v, ok := bindingCache.Load(t); ok {
		return v.(*bindings)
	}
	b := buildBindings(t)
	v, _ := bindingCache.LoadOrStore(t, b)
	return v.(*bindings)
}

func buildBindings(t reflect.Type) *bindings {
	b := &bindings{byCol: make(map[string]int)}
	walkBindings(nil, t, b)
	for i, f := range b.fields {
		if _, dup := b.byCol[f.column]; !dup {
			b.byCol[f.column] = i
		} else {
			b.byCol[f.column] = -1 // later duplicates lose; first wins deterministically
		}
	}
	return b
}

func walkBindings(prefix []int, t reflect.Type, b *bindings) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		path := make([]int, len(prefix)+1)
		copy(path, prefix)
		path[len(prefix)] = i
		// Embedded structs flatten into the parent plan — including
		// unexported embedded types, whose exported fields remain settable
		// (same rule as encoding/json).
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			walkBindings(path, f.Type, b)
			continue
		}
		if f.PkgPath != "" {
			continue // unexported leaf field: not settable
		}
		b.fields = append(b.fields, fieldBinding{
			path:   path,
			column: columnName(f),
			name:   f.Name,
		})
	}
}

func mapToStructValue(row map[string]any, dest reflect.Value) error {
	b := bindingsOf(dest.Type())
	return b.applyMap(row, dest)
}

func (b *bindings) applyMap(row map[string]any, dest reflect.Value) error {
	for i := range b.fields {
		fb := &b.fields[i]
		val, ok := row[fb.column]
		if !ok {
			continue
		}
		if err := setField(dest.FieldByIndex(fb.path), val); err != nil {
			return fmt.Errorf("blinq/scan: field %s: %w", fb.name, err)
		}
	}
	return nil
}

func columnName(field reflect.StructField) string {
	if tag := field.Tag.Get("db"); tag != "" && tag != "-" {
		if i := strings.IndexByte(tag, ','); i >= 0 {
			tag = tag[:i]
		}
		if tag != "" {
			return tag
		}
	}
	return ToSnakeCase(field.Name)
}

func setField(field reflect.Value, val any) error {
	if !field.CanSet() {
		return nil
	}
	if val == nil {
		if field.Kind() == reflect.Ptr {
			field.Set(reflect.Zero(field.Type()))
		}
		return nil
	}

	// *T where *T implements sql.Scanner (e.g. *geo.Latitude / *geo.Longitude).
	if field.Kind() == reflect.Ptr {
		elemType := field.Type().Elem()
		if elemType == reflect.TypeOf(time.Time{}) {
			t := AsTime(val)
			field.Set(reflect.ValueOf(&t))
			return nil
		}
		if field.IsNil() {
			field.Set(reflect.New(elemType))
		}
		if sc, ok := field.Interface().(interface{ Scan(any) error }); ok {
			return sc.Scan(scanValueForCustom(val))
		}
		switch elemType.Kind() {
		case reflect.Float64:
			f := AsFloat64(val)
			field.Set(reflect.ValueOf(&f))
			return nil
		case reflect.Float32:
			f := float32(AsFloat64(val))
			field.Set(reflect.ValueOf(&f))
			return nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n := AsInt64(val)
			ptr := reflect.New(elemType)
			ptr.Elem().SetInt(n)
			field.Set(ptr)
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n := uint64(AsInt64(val))
			ptr := reflect.New(elemType)
			ptr.Elem().SetUint(n)
			field.Set(ptr)
			return nil
		case reflect.String:
			s := AsString(val)
			field.Set(reflect.ValueOf(&s))
			return nil
		case reflect.Bool:
			b := AsBool(val)
			field.Set(reflect.ValueOf(&b))
			return nil
		}
	}

	// T where *T implements sql.Scanner (JSONB, geo.Latitude value, etc.).
	if field.CanAddr() {
		if sc, ok := field.Addr().Interface().(interface{ Scan(any) error }); ok {
			return sc.Scan(scanValueForCustom(val))
		}
	}

	// Direct assignment when types match.
	rv := reflect.ValueOf(val)
	if rv.IsValid() && rv.Type().AssignableTo(field.Type()) {
		field.Set(rv)
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(AsString(val))
	case reflect.Bool:
		field.SetBool(AsBool(val))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.SetInt(AsInt64(val))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		field.SetUint(uint64(AsInt64(val)))
	case reflect.Float32, reflect.Float64:
		field.SetFloat(AsFloat64(val))
	case reflect.Struct:
		if field.Type() == reflect.TypeOf(time.Time{}) {
			field.Set(reflect.ValueOf(AsTime(val)))
			return nil
		}
		fallthrough
	default:
		if rv.IsValid() && rv.Type().ConvertibleTo(field.Type()) {
			field.Set(rv.Convert(field.Type()))
			return nil
		}
		return fmt.Errorf("unsupported type %s for value %T", field.Type(), val)
	}
	return nil
}

// scanValueForCustom normalizes driver types (e.g. pgtype.Numeric) to
// values that generic sql.Scanner implementations usually accept (float64).
func scanValueForCustom(val any) any {
	if AsFloat64Hook != nil {
		if f, ok := AsFloat64Hook(val); ok {
			return f
		}
	}
	return val
}

// RowCursor is the minimal contract for direct cursor→struct scanning
// (satisfied by SQL drivers' Rows wrappers that expose columns + values).
type RowCursor interface {
	Columns() []string
	Values() ([]any, error)
	Next() bool
	Err() error
	Close()
}

// ScanCursorInto scans every row of cursor into *[]T without materializing
// intermediate []map[string]any. Column matching follows the same rules as
// MapToStruct (`db` tag or snake_case of the field name).
func ScanCursorInto[T any](cursor RowCursor, dest *[]T) error {
	out := make([]T, 0, 16)
	var b *bindings
	var assign func([]any, *T) error
	for cursor.Next() {
		values, err := cursor.Values()
		if err != nil {
			return err
		}
		if b == nil {
			var zero T
			b = bindingsOf(reflect.TypeOf(zero))
			raw := b.assigner(cursor.Columns(), len(values))
			assign = func(vals []any, item *T) error { return raw(vals, item) }
		}
		var item T
		if err := assign(values, &item); err != nil {
			return err
		}
		out = append(out, item)
	}
	if err := cursor.Err(); err != nil {
		return err
	}
	*dest = out
	return nil
}

// assigner pre-resolves column positions against this plan and returns a
// function assigning positional values into a struct instance.
func (b *bindings) assigner(cols []string, nValues int) func([]any, any) error {
	pos := make([]int, 0, len(cols)) // value index
	idx := make([]int, 0, len(cols)) // binding index
	matched := make(map[int]bool, nValues)
	for ci, col := range cols {
		if ci >= nValues {
			break
		}
		bi, ok := b.byCol[col]
		if !ok || bi < 0 || matched[bi] {
			continue // unknown column / ambiguous / already bound (first wins)
		}
		matched[bi] = true
		pos = append(pos, ci)
		idx = append(idx, bi)
	}
	return func(values []any, destPtr any) error {
		rv := reflect.ValueOf(destPtr).Elem()
		for k, bi := range idx {
			fb := &b.fields[bi]
			if pos[k] >= len(values) {
				continue
			}
			if err := setField(rv.FieldByIndex(fb.path), values[pos[k]]); err != nil {
				return fmt.Errorf("blinq/scan: field %s: %w", fb.name, err)
			}
		}
		return nil
	}
}

// ToSnakeCase converts CamelCase / ID → snake_case.
func ToSnakeCase(s string) string {
	if isASCII(s) {
		return snakeASCII(s)
	}
	var result []rune
	runes := []rune(s)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				prev := runes[i-1]
				if prev >= 'a' && prev <= 'z' {
					result = append(result, '_')
				} else if prev >= 'A' && prev <= 'Z' && i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z' {
					result = append(result, '_')
				}
			}
			result = append(result, r-'A'+'a')
			continue
		}
		result = append(result, r)
	}
	return string(result)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7f {
			return false
		}
	}
	return true
}

func snakeASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	prevUpper := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			if i > 0 {
				prev := s[i-1]
				nextLower := i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z'
				if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z' && nextLower) {
					b.WriteByte('_')
				}
			}
			b.WriteByte(c + 'a' - 'A')
			prevUpper = true
		default:
			b.WriteByte(c)
			prevUpper = false
		}
	}
	_ = prevUpper
	return b.String()
}
