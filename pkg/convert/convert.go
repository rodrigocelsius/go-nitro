// Package convert provides a unified conversion API for Nitro portable types.
package convert

import (
	"fmt"
	"reflect"

	"github.com/rodrigocelsius/go-nitro/domain/v2/decimal"
	"github.com/rodrigocelsius/go-nitro/domain/v2/geo"
	"github.com/rodrigocelsius/go-nitro/domain/v2/locale"
	"github.com/rodrigocelsius/go-nitro/domain/v2/measure"
	"github.com/rodrigocelsius/go-nitro/domain/v2/money"
	"github.com/rodrigocelsius/go-nitro/domain/v2/temporal"
	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

// Option configures conversion.
type Option func(*options)

type options struct {
	locale string
	unit   measure.Unit
}

// WithLocale sets locale for display parse/format.
func WithLocale(loc string) Option {
	return func(o *options) { o.locale = loc }
}

// WithUnit sets target unit for Measure conversion.
func WithUnit(u measure.Unit) Option {
	return func(o *options) { o.unit = u }
}

var (
	ErrInvalidFormat     = fmt.Errorf("convert: invalid format")
	ErrIncompatibleUnit  = measure.ErrIncompatibleUnit
	ErrUnsupportedTarget = fmt.Errorf("convert: unsupported target")
)

// To converts from into type T using options.
func To[T any](from any, opts ...Option) (T, error) {
	var zero T
	o := options{locale: locale.Locale()}
	for _, fn := range opts {
		fn(&o)
	}
	target := reflect.TypeOf(zero)
	if target == nil {
		return zero, ErrUnsupportedTarget
	}

	switch any(zero).(type) {
	case nitrouuid.UUID:
		u, err := toUUID(from)
		if err != nil {
			return zero, err
		}
		return any(u).(T), nil
	case temporal.Date:
		d, err := toDate(from, o.locale)
		if err != nil {
			return zero, err
		}
		return any(d).(T), nil
	case temporal.TimeOfDay:
		t, err := toTimeOfDay(from)
		if err != nil {
			return zero, err
		}
		return any(t).(T), nil
	case temporal.DateTime:
		dt, err := toDateTime(from, o.locale)
		if err != nil {
			return zero, err
		}
		return any(dt).(T), nil
	case money.Money:
		m, err := toMoney(from, o.locale)
		if err != nil {
			return zero, err
		}
		return any(m).(T), nil
	case measure.Measure:
		m, err := toMeasure(from, o)
		if err != nil {
			return zero, err
		}
		return any(m).(T), nil
	case decimal.Decimal:
		d, err := toDecimal(from)
		if err != nil {
			return zero, err
		}
		return any(d).(T), nil
	case geo.LatLng:
		p, err := toLatLng(from)
		if err != nil {
			return zero, err
		}
		return any(p).(T), nil
	case geo.Latitude:
		la, err := toLatitude(from)
		if err != nil {
			return zero, err
		}
		return any(la).(T), nil
	case geo.Longitude:
		lo, err := toLongitude(from)
		if err != nil {
			return zero, err
		}
		return any(lo).(T), nil
	case string:
		s, err := toString(from, o.locale)
		if err != nil {
			return zero, err
		}
		return any(s).(T), nil
	default:
		return zero, fmt.Errorf("%w: %T", ErrUnsupportedTarget, zero)
	}
}

// Must is To that panics on error.
func Must[T any](from any, opts ...Option) T {
	v, err := To[T](from, opts...)
	if err != nil {
		panic(err)
	}
	return v
}

func toUUID(from any) (nitrouuid.UUID, error) {
	switch v := from.(type) {
	case nitrouuid.UUID:
		return v, nil
	case string:
		return nitrouuid.Parse(v)
	case []byte:
		if len(v) == nitrouuid.Size {
			return nitrouuid.FromBytes(v)
		}
		return nitrouuid.Parse(string(v))
	default:
		return nitrouuid.UUID{}, ErrInvalidFormat
	}
}

func toDate(from any, loc string) (temporal.Date, error) {
	switch v := from.(type) {
	case temporal.Date:
		return v, nil
	case temporal.DateTime:
		d, _ := temporal.SplitLocal(v, locale.Timezone())
		return d, nil
	case string:
		return temporal.ParseDate(v, loc)
	default:
		return temporal.Date{}, ErrInvalidFormat
	}
}

func toTimeOfDay(from any) (temporal.TimeOfDay, error) {
	switch v := from.(type) {
	case temporal.TimeOfDay:
		return v, nil
	case temporal.DateTime:
		_, tod := temporal.SplitLocal(v, locale.Timezone())
		return tod, nil
	case string:
		return temporal.ParseTimeOfDayISO(v)
	default:
		return temporal.TimeOfDay{}, ErrInvalidFormat
	}
}

func toDateTime(from any, loc string) (temporal.DateTime, error) {
	switch v := from.(type) {
	case temporal.DateTime:
		return v, nil
	case temporal.Date:
		return temporal.CombineLocal(v, temporal.TimeOfDay{}, locale.Timezone()), nil
	case string:
		return temporal.ParseDateTime(v, loc)
	default:
		return temporal.DateTime{}, ErrInvalidFormat
	}
}

func toMoney(from any, loc string) (money.Money, error) {
	switch v := from.(type) {
	case money.Money:
		return v, nil
	case string:
		return money.Parse(v, "", loc)
	default:
		return money.Money{}, ErrInvalidFormat
	}
}

func toMeasure(from any, o options) (measure.Measure, error) {
	switch v := from.(type) {
	case measure.Measure:
		if o.unit != "" && o.unit != v.Unit {
			return measure.Convert(v, o.unit)
		}
		return v, nil
	case string:
		m, err := measure.ParseMeasure(v, o.locale)
		if err != nil {
			return measure.Measure{}, err
		}
		if o.unit != "" && o.unit != m.Unit {
			return measure.Convert(m, o.unit)
		}
		return m, nil
	default:
		return measure.Measure{}, ErrInvalidFormat
	}
}

func toDecimal(from any) (decimal.Decimal, error) {
	switch v := from.(type) {
	case decimal.Decimal:
		return v, nil
	case string:
		return decimal.NewFromString(v)
	case float64:
		return decimal.NewFromFloat64(v), nil
	case int64:
		return decimal.NewFromInt64(v), nil
	case int:
		return decimal.NewFromInt64(int64(v)), nil
	default:
		return decimal.Decimal{}, ErrInvalidFormat
	}
}

func toLatLng(from any) (geo.LatLng, error) {
	switch v := from.(type) {
	case geo.LatLng:
		return v, nil
	case string:
		return geo.ParseLatLng(v)
	default:
		return geo.LatLng{}, ErrInvalidFormat
	}
}

func toLatitude(from any) (geo.Latitude, error) {
	switch v := from.(type) {
	case geo.Latitude:
		return v, nil
	case float64:
		return geo.NewLatitude(v)
	case string:
		return geo.ParseLatitude(v)
	default:
		return 0, ErrInvalidFormat
	}
}

func toLongitude(from any) (geo.Longitude, error) {
	switch v := from.(type) {
	case geo.Longitude:
		return v, nil
	case float64:
		return geo.NewLongitude(v)
	case string:
		return geo.ParseLongitude(v)
	default:
		return 0, ErrInvalidFormat
	}
}

func toString(from any, loc string) (string, error) {
	switch v := from.(type) {
	case string:
		return v, nil
	case nitrouuid.UUID:
		return v.String(), nil
	case temporal.Date:
		return temporal.FormatDate(v, loc), nil
	case temporal.DateTime:
		return temporal.FormatDateTime(v, loc), nil
	case temporal.TimeOfDay:
		return v.String(), nil
	case money.Money:
		return money.Format(v, loc), nil
	case measure.Measure:
		return measure.FormatMeasure(v, loc), nil
	case decimal.Decimal:
		return v.String(), nil
	case geo.LatLng:
		return geo.FormatLatLng(v, loc), nil
	case geo.Latitude:
		return v.String(), nil
	case geo.Longitude:
		return v.String(), nil
	default:
		return fmt.Sprint(from), nil
	}
}
