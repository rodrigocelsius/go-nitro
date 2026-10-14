package domain

import (
	"reflect"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2/temporal"
	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

// EntityDefaults controls filling of ID (UUID v7) and timestamps.
type EntityDefaults struct {
	AutoID         bool
	AutoTimestamps bool
}

// ApplyEntityDefaults fills an empty ID and CreatedAt/UpdatedAt.
// Used by Resource, MemoryRepo and pgx.Store (Create/Update parity).
// ID accepts string or uuid.UUID; timestamps accept time.Time or temporal.DateTime.
func ApplyEntityDefaults(entity any, d EntityDefaults, isCreate bool) {
	rv := reflect.ValueOf(entity)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}

	now := time.Now().UTC()
	nowDT := temporal.DateTimeFrom(now)

	if d.AutoID && isCreate {
		setIDIfEmpty(rv)
	}

	if d.AutoTimestamps {
		if isCreate {
			setTimeIfZero(rv, "CreatedAt", now, nowDT)
			setTimeIfZero(rv, "UpdatedAt", now, nowDT)
		} else {
			setTimeField(rv, "UpdatedAt", now, nowDT)
		}
	}
}

func setIDIfEmpty(rv reflect.Value) {
	f := FieldByName(rv, "ID")
	if !f.IsValid() || !f.CanSet() {
		return
	}
	switch f.Kind() {
	case reflect.String:
		if f.String() == "" {
			f.SetString(nitrouuid.NewString())
		}
	default:
		if f.Type() == reflect.TypeOf(nitrouuid.UUID{}) {
			if f.Interface().(nitrouuid.UUID).IsZero() {
				f.Set(reflect.ValueOf(nitrouuid.New()))
			}
		}
	}
}

func setTimeIfZero(rv reflect.Value, name string, t time.Time, dt temporal.DateTime) {
	f := FieldByName(rv, name)
	if !f.IsValid() || !f.CanAddr() || !f.CanSet() {
		return
	}
	// Work through the field's address: Interface() on a struct value boxes
	// (allocates) on every request.
	switch p := f.Addr().Interface().(type) {
	case *time.Time:
		if p.IsZero() {
			*p = t
		}
	case **time.Time:
		if *p == nil {
			tt := t
			*p = &tt
		}
	case *temporal.DateTime:
		if p.IsZero() {
			*p = dt
		}
	case **temporal.DateTime:
		if *p == nil {
			d := dt
			*p = &d
		}
	}
}

func setTimeField(rv reflect.Value, name string, t time.Time, dt temporal.DateTime) {
	f := FieldByName(rv, name)
	if !f.IsValid() || !f.CanAddr() || !f.CanSet() {
		return
	}
	switch p := f.Addr().Interface().(type) {
	case *time.Time:
		*p = t
	case **time.Time:
		tt := t
		*p = &tt
	case *temporal.DateTime:
		*p = dt
	case **temporal.DateTime:
		d := dt
		*p = &d
	}
}
