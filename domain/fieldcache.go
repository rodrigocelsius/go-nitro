package domain

import (
	"reflect"
	"sync"
)

type fieldKey struct {
	t    reflect.Type
	name string
}

// fieldIndexes caches reflect index paths: FieldByName on a struct with
// embedded fields (nitro.Model, TenantModel) walks and allocates on every call,
// which dominated Resource/MemoryRepo request profiles.
var fieldIndexes sync.Map // fieldKey → []int (nil: no such field)

// FieldByName is reflect.Value.FieldByName with a per-type index cache. It
// returns the zero Value when v is not a struct, the field does not exist, or
// it sits behind a nil embedded pointer (where reflect would panic).
func FieldByName(v reflect.Value, name string) reflect.Value {
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	key := fieldKey{v.Type(), name}
	idx, ok := fieldIndexes.Load(key)
	if !ok {
		var path []int
		if sf, found := key.t.FieldByName(name); found {
			path = sf.Index
		}
		idx, _ = fieldIndexes.LoadOrStore(key, path)
	}
	path := idx.([]int)
	if path == nil {
		return reflect.Value{}
	}
	if len(path) == 1 {
		return v.Field(path[0])
	}
	f, err := v.FieldByIndexErr(path)
	if err != nil {
		return reflect.Value{}
	}
	return f
}
