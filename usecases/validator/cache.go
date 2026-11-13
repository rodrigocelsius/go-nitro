package validator

import (
	"reflect"
	"strings"
	"sync"
)

// typeMeta is cached metadata per reflect.Type (exported fields + parsed tags).
type typeMeta struct {
	fields []cachedField
}

type cachedField struct {
	index    int
	field    reflect.StructField
	jsonName string
	// flatten: an embedded struct without a JSON name. encoding/json promotes
	// its fields to the parent, so their errors use the parent's path.
	flatten    bool
	hasTag     bool
	meta       validateMeta
	fieldRules []validateRule
	diveRules  []validateRule
}

var typeMetaCache sync.Map // map[reflect.Type]*typeMeta

func lookupTypeMeta(t reflect.Type) *typeMeta {
	if t == nil {
		return &typeMeta{}
	}
	if v, ok := typeMetaCache.Load(t); ok {
		return v.(*typeMeta)
	}
	m := buildTypeMeta(t)
	actual, _ := typeMetaCache.LoadOrStore(t, m)
	return actual.(*typeMeta)
}

func buildTypeMeta(t reflect.Type) *typeMeta {
	n := t.NumField()
	fields := make([]cachedField, 0, n)
	for i := 0; i < n; i++ {
		sf := t.Field(i)
		flatten := sf.Anonymous && embeddedStruct(sf.Type) && !hasJSONName(sf)
		// Unexported fields are skipped, except embedded structs: their
		// exported fields are promoted (and decoded) like the parent's own.
		if sf.PkgPath != "" && !(flatten && sf.Type.Kind() == reflect.Struct) {
			continue
		}
		cf := cachedField{
			index:    i,
			field:    sf,
			jsonName: jsonFieldName(sf),
			flatten:  flatten,
		}
		tag := sf.Tag.Get("validate")
		if tag != "" {
			cf.hasTag = true
			cf.meta, cf.fieldRules, cf.diveRules = parseValidateTag(tag)
		}
		fields = append(fields, cf)
	}
	return &typeMeta{fields: fields}
}

func embeddedStruct(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}

func hasJSONName(sf reflect.StructField) bool {
	n, _, _ := strings.Cut(sf.Tag.Get("json"), ",")
	return n != "" && n != "-"
}

// ClearTypeMetaCache limpa o cache (testes).
func ClearTypeMetaCache() {
	typeMetaCache = sync.Map{}
}
