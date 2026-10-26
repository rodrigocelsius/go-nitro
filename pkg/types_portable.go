package nitro

import (
	"github.com/rodrigocelsius/go-nitro/domain/v2/decimal"
	"github.com/rodrigocelsius/go-nitro/domain/v2/geo"
	"github.com/rodrigocelsius/go-nitro/domain/v2/locale"
	"github.com/rodrigocelsius/go-nitro/domain/v2/measure"
	"github.com/rodrigocelsius/go-nitro/domain/v2/money"
	"github.com/rodrigocelsius/go-nitro/domain/v2/temporal"
	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

// Portable column types (Scan/Value + JSON). Prefer these over string / time.Time in models.

// UUID is a UUID v7 (and general) identifier type.
type UUID = nitrouuid.UUID

// Date is a civil calendar day (JSON "YYYY-MM-DD").
type Date = temporal.Date

// TimeOfDay is a clock time without date (JSON "HH:mm:ss").
type TimeOfDay = temporal.TimeOfDay

// DateTime is a UTC instant (JSON RFC3339). Replaces time.Time in Model.
type DateTime = temporal.DateTime

// Money is minor-units amount + ISO 4217 currency.
type Money = money.Money

// Measure is a value with a physical unit (m, ft, yd, …).
type Measure = measure.Measure

// Unit is a measure unit code.
type Unit = measure.Unit

// Decimal is an arbitrary-precision decimal.
type Decimal = decimal.Decimal

// Latitude / Longitude / LatLng are WGS84 coords (float64, max 9 fractional digits).
type Latitude = geo.Latitude

// Longitude is a WGS84 longitude (see Latitude).
type Longitude = geo.Longitude

// LatLng is a WGS84 coordinate pair (see Latitude).
type LatLng = geo.LatLng

// MaxFracDigits is the canonical geo decimal scale.
const MaxFracDigits = geo.MaxFracDigits

// Unit constants (length / mass / …).
const (
	Meter      = measure.Meter
	Centimeter = measure.Centimeter
	Kilometer  = measure.Kilometer
	Inch       = measure.Inch
	Foot       = measure.Foot
	Yard       = measure.Yard
	Mile       = measure.Mile
	Kilogram   = measure.Kilogram
	Gram       = measure.Gram
	Pound      = measure.Pound
	Ounce      = measure.Ounce
	Celsius    = measure.Celsius
	Fahrenheit = measure.Fahrenheit
	Kelvin     = measure.Kelvin
	Liter      = measure.Liter
	GallonUS   = measure.GallonUS
)

// Temporal helpers.
var (
	Now                   = temporal.Now
	DateTimeFrom          = temporal.DateTimeFrom
	NewDate               = temporal.NewDate
	NewTimeOfDay          = temporal.NewTimeOfDay
	ParseDateISO          = temporal.ParseDateISO
	MustParseDateISO      = temporal.MustParseDateISO
	ParseTimeOfDayISO     = temporal.ParseTimeOfDayISO
	MustParseTimeOfDayISO = temporal.MustParseTimeOfDayISO
	CombineLocal          = temporal.CombineLocal
	SplitLocal            = temporal.SplitLocal
	FormatDate            = temporal.FormatDate
	FormatDateTime        = temporal.FormatDateTime
	ParseDate             = temporal.ParseDate
	ParseDateTime         = temporal.ParseDateTime
)

// Money helpers.
var (
	NewMoney       = money.New
	MoneyFromMajor = money.FromMajor
	FormatMoney    = money.Format
	ParseMoney     = money.Parse
)

// Measure helpers.
var (
	Meters         = measure.Meters
	Feet           = measure.Feet
	Yards          = measure.Yards
	ConvertMeasure = measure.Convert
	MustConvert    = measure.MustConvert
	FormatMeasure  = measure.FormatMeasure
	ParseMeasure   = measure.ParseMeasure
	RegisterUnit   = measure.RegisterUnit
)

// Locale helpers.
var (
	SetLocale   = locale.SetLocale
	Locale      = locale.Locale
	WithLocale  = locale.WithLocale
	SetTimezone = locale.SetTimezone
	Timezone    = locale.Timezone
)

// Decimal helpers.
var (
	NewDecimalFromString  = decimal.NewFromString
	NewDecimalFromFloat64 = decimal.NewFromFloat64
	NewDecimalFromInt64   = decimal.NewFromInt64
)

// Geo helpers.
var (
	NewLatitude    = geo.NewLatitude
	NewLongitude   = geo.NewLongitude
	NewLatLng      = geo.NewLatLng
	MustLatLng     = geo.MustLatLng
	ParseLatLng    = geo.ParseLatLng
	ParseLatitude  = geo.ParseLatitude
	ParseLongitude = geo.ParseLongitude
	DistanceMeters = geo.DistanceMeters
	FormatLatLng   = geo.FormatLatLng
)

// UUID helpers (also in ids.go — ParseUUID / MustParseUUID / NewUUID).
