package domain

import (
	"reflect"
	"strings"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2/temporal"
)

// SoftDeleteColumn returns the soft-delete column name if the type (or an
// anonymous embed) has DeletedAt / db:"deleted_at". Otherwise "".
//
// Used by MemoryRepo and pgx.Store for the same criterion.
func SoftDeleteColumn(t reflect.Type) string {
	if t == nil {
		return ""
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return ""
	}
	return findSoftColumn(t)
}

func findSoftColumn(t reflect.Type) string {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		if f.Anonymous {
			ft := f.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				if col := findSoftColumn(ft); col != "" {
					return col
				}
			}
			continue
		}
		db := f.Tag.Get("db")
		if j := strings.IndexByte(db, ','); j >= 0 {
			db = db[:j]
		}
		if db == "deleted_at" || f.Name == "DeletedAt" {
			if isSoftDeleteType(f.Type) {
				if db != "" {
					return db
				}
				return "deleted_at"
			}
		}
	}
	return ""
}

func isSoftDeleteType(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == reflect.TypeOf(time.Time{}) {
		return true
	}
	if t == reflect.TypeOf(temporal.DateTime{}) {
		return true
	}
	if t.Kind() == reflect.Struct {
		if _, ok := t.FieldByName("Valid"); ok {
			if _, ok := t.FieldByName("Time"); ok {
				return true
			}
		}
	}
	return false
}

// HasSoftDelete reports whether T uses soft delete.
func HasSoftDelete[T any]() bool {
	var zero T
	return SoftDeleteColumn(reflect.TypeOf(zero)) != ""
}

// SoftDeleteSQL returns the default SQL predicate for rows that are not deleted
// (e.g. "deleted_at IS NULL"), or "" if the type has no soft delete.
//
// Soft delete (DeletedAt) is independent of business status (active/inactive).
// Use Filterable/FilterRules for status; Store already excludes soft-deleted rows automatically.
func SoftDeleteSQL(t reflect.Type) string {
	col := SoftDeleteColumn(t)
	if col == "" {
		return ""
	}
	return col + " IS NULL"
}

// SoftDeleteSQLOf is the typed SoftDeleteSQL.
func SoftDeleteSQLOf[T any]() string {
	var zero T
	return SoftDeleteSQL(reflect.TypeOf(zero))
}

// ModelIDIsInteger reports whether the promoted ID field is numeric (BIGSERIAL).
func ModelIDIsInteger(t reflect.Type) bool {
	if t == nil {
		return false
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	f, ok := t.FieldByName("ID")
	if !ok {
		return false
	}
	switch f.Type.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}
