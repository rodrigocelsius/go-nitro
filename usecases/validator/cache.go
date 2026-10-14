package validator

import (
	"reflect"
	"sync"
)

// typeMeta is cached metadata per reflect.Type (exported fields + parsed tags).
type typeMeta struct {
	fields []cachedField
}

type cachedField struct {
	index      int
	field      reflect.StructField
	jsonName   string
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
		if sf.PkgPath != "" {
			continue
		}
		cf := cachedField{
			index:    i,
			field:    sf,
			jsonName: jsonFieldName(sf),
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

// ClearTypeMetaCache limpa o cache (testes).
func ClearTypeMetaCache() {
	typeMetaCache = sync.Map{}
}
