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
	f := rv.FieldByName("ID")
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
	f := rv.FieldByName(name)
	if !f.IsValid() || !f.CanSet() {
		return
	}
	switch f.Type() {
	case reflect.TypeOf(time.Time{}):
		if f.Interface().(time.Time).IsZero() {
			f.Set(reflect.ValueOf(t))
		}
	case reflect.TypeOf((*time.Time)(nil)):
		if f.IsNil() {
			f.Set(reflect.ValueOf(&t))
		}
	case reflect.TypeOf(temporal.DateTime{}):
		if f.Interface().(temporal.DateTime).IsZero() {
			f.Set(reflect.ValueOf(dt))
		}
	case reflect.TypeOf((*temporal.DateTime)(nil)):
		if f.IsNil() {
			f.Set(reflect.ValueOf(&dt))
		}
	}
}

func setTimeField(rv reflect.Value, name string, t time.Time, dt temporal.DateTime) {
	f := rv.FieldByName(name)
	if !f.IsValid() || !f.CanSet() {
		return
	}
	switch f.Type() {
	case reflect.TypeOf(time.Time{}):
		f.Set(reflect.ValueOf(t))
	case reflect.TypeOf((*time.Time)(nil)):
		f.Set(reflect.ValueOf(&t))
	case reflect.TypeOf(temporal.DateTime{}):
		f.Set(reflect.ValueOf(dt))
	case reflect.TypeOf((*temporal.DateTime)(nil)):
		f.Set(reflect.ValueOf(&dt))
	}
}
