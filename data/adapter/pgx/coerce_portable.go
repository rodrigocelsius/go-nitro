package pgx

import (
	"reflect"

	"github.com/rodrigocelsius/go-nitro/domain/v2/decimal"
	"github.com/rodrigocelsius/go-nitro/domain/v2/geo"
	"github.com/rodrigocelsius/go-nitro/domain/v2/measure"
	"github.com/rodrigocelsius/go-nitro/domain/v2/money"
	"github.com/rodrigocelsius/go-nitro/domain/v2/temporal"
	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

func coercePortable(t reflect.Type, s string) any {
	switch t {
	case reflect.TypeOf(nitrouuid.UUID{}):
		if u, err := nitrouuid.Parse(s); err == nil {
			return u
		}
	case reflect.TypeOf(temporal.Date{}):
		if d, err := temporal.ParseDateISO(s); err == nil {
			return d
		}
		if d, err := temporal.ParseDate(s, ""); err == nil {
			return d
		}
	case reflect.TypeOf(temporal.TimeOfDay{}):
		if tod, err := temporal.ParseTimeOfDayISO(s); err == nil {
			return tod
		}
	case reflect.TypeOf(temporal.DateTime{}):
		if dt, err := temporal.ParseDateTimeRFC3339(s); err == nil {
			return dt
		}
		if dt, err := temporal.ParseDateTime(s, ""); err == nil {
			return dt
		}
	case reflect.TypeOf(money.Money{}):
		if m, err := money.Parse(s, "", ""); err == nil {
			return m
		}
	case reflect.TypeOf(measure.Measure{}):
		if m, err := measure.ParseMeasure(s, ""); err == nil {
			return m
		}
	case reflect.TypeOf(decimal.Decimal{}):
		if d, err := decimal.NewFromString(s); err == nil {
			return d
		}
	case reflect.TypeOf(geo.Latitude(0)):
		if la, err := geo.ParseLatitude(s); err == nil {
			return la
		}
	case reflect.TypeOf(geo.Longitude(0)):
		if lo, err := geo.ParseLongitude(s); err == nil {
			return lo
		}
	case reflect.TypeOf(geo.LatLng{}):
		if p, err := geo.ParseLatLng(s); err == nil {
			return p
		}
	}
	return s
}
