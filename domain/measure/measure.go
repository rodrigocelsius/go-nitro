package measure

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Unit is a measurement unit code (e.g. "m", "ft", "yd").
type Unit string

const (
	Meter      Unit = "m"
	Centimeter Unit = "cm"
	Kilometer  Unit = "km"
	Inch       Unit = "in"
	Foot       Unit = "ft"
	Yard       Unit = "yd"
	Mile       Unit = "mi"

	Kilogram Unit = "kg"
	Gram     Unit = "g"
	Pound    Unit = "lb"
	Ounce    Unit = "oz"

	Celsius    Unit = "C"
	Fahrenheit Unit = "F"
	Kelvin     Unit = "K"

	Liter    Unit = "L"
	GallonUS Unit = "gal"
	PintUS   Unit = "pt"

	SquareMeter Unit = "m2"
	SquareFoot  Unit = "ft2"
	SquareYard  Unit = "yd2"
	Acre        Unit = "acre"
)

// Dimension groups compatible units.
type Dimension string

const (
	DimLength      Dimension = "length"
	DimMass        Dimension = "mass"
	DimTemperature Dimension = "temperature"
	DimVolume      Dimension = "volume"
	DimArea        Dimension = "area"
)

type unitInfo struct {
	Dim    Dimension
	ToSI   func(float64) float64 // value in this unit → SI
	FromSI func(float64) float64 // SI → this unit
}

var registry = map[Unit]unitInfo{}

func init() {
	registerLinear(Meter, DimLength, 1)
	registerLinear(Centimeter, DimLength, 0.01)
	registerLinear(Kilometer, DimLength, 1000)
	registerLinear(Inch, DimLength, 0.0254)
	registerLinear(Foot, DimLength, 0.3048)
	registerLinear(Yard, DimLength, 0.9144)
	registerLinear(Mile, DimLength, 1609.344)

	registerLinear(Kilogram, DimMass, 1)
	registerLinear(Gram, DimMass, 0.001)
	registerLinear(Pound, DimMass, 0.45359237)
	registerLinear(Ounce, DimMass, 0.028349523125)

	registerLinear(Liter, DimVolume, 1)
	registerLinear(GallonUS, DimVolume, 3.785411784)
	registerLinear(PintUS, DimVolume, 0.473176473)

	registerLinear(SquareMeter, DimArea, 1)
	registerLinear(SquareFoot, DimArea, 0.09290304)
	registerLinear(SquareYard, DimArea, 0.83612736)
	registerLinear(Acre, DimArea, 4046.8564224)

	registry[Celsius] = unitInfo{
		Dim:    DimTemperature,
		ToSI:   func(v float64) float64 { return v }, // SI storage = Celsius
		FromSI: func(v float64) float64 { return v },
	}
	registry[Fahrenheit] = unitInfo{
		Dim:    DimTemperature,
		ToSI:   func(v float64) float64 { return (v - 32) * 5 / 9 },
		FromSI: func(v float64) float64 { return v*9/5 + 32 },
	}
	registry[Kelvin] = unitInfo{
		Dim:    DimTemperature,
		ToSI:   func(v float64) float64 { return v - 273.15 },
		FromSI: func(v float64) float64 { return v + 273.15 },
	}
}

func registerLinear(u Unit, dim Dimension, factor float64) {
	f := factor
	registry[u] = unitInfo{
		Dim:    dim,
		ToSI:   func(v float64) float64 { return v * f },
		FromSI: func(v float64) float64 { return v / f },
	}
}

// RegisterUnit adds or overrides a unit (apps can extend).
func RegisterUnit(u Unit, dim Dimension, toSI, fromSI func(float64) float64) {
	registry[u] = unitInfo{Dim: dim, ToSI: toSI, FromSI: fromSI}
}

// Measure is a numeric value with a unit.
type Measure struct {
	Val  float64 `json:"value"`
	Unit Unit    `json:"unit"`
}

// New builds a Measure.
func New(value float64, unit Unit) Measure {
	return Measure{Val: value, Unit: unit}
}

// Meters / Feet / Yards helpers.
func Meters(v float64) Measure    { return New(v, Meter) }
func Feet(v float64) Measure      { return New(v, Foot) }
func Yards(v float64) Measure     { return New(v, Yard) }
func Kilograms(v float64) Measure { return New(v, Kilogram) }

// ErrIncompatibleUnit is returned when converting across dimensions.
var ErrIncompatibleUnit = fmt.Errorf("measure: incompatible unit")

// Convert converts m to toUnit.
func Convert(m Measure, to Unit) (Measure, error) {
	fromInfo, ok := registry[m.Unit]
	if !ok {
		return Measure{}, fmt.Errorf("measure: unknown unit %q", m.Unit)
	}
	toInfo, ok := registry[to]
	if !ok {
		return Measure{}, fmt.Errorf("measure: unknown unit %q", to)
	}
	if fromInfo.Dim != toInfo.Dim {
		return Measure{}, ErrIncompatibleUnit
	}
	si := fromInfo.ToSI(m.Val)
	return Measure{Val: toInfo.FromSI(si), Unit: to}, nil
}

// MustConvert panics on error.
func MustConvert(m Measure, to Unit) Measure {
	out, err := Convert(m, to)
	if err != nil {
		panic(err)
	}
	return out
}

// ToSI converts to the dimension's SI unit.
func (m Measure) ToSI() (Measure, error) {
	info, ok := registry[m.Unit]
	if !ok {
		return Measure{}, fmt.Errorf("measure: unknown unit %q", m.Unit)
	}
	var si Unit
	switch info.Dim {
	case DimLength:
		si = Meter
	case DimMass:
		si = Kilogram
	case DimTemperature:
		si = Celsius
	case DimVolume:
		si = Liter
	case DimArea:
		si = SquareMeter
	default:
		return Measure{}, fmt.Errorf("measure: unknown dimension")
	}
	return Convert(m, si)
}

// EqualApprox compares with relative tolerance.
func (m Measure) EqualApprox(o Measure, eps float64) bool {
	a, err := Convert(m, o.Unit)
	if err != nil {
		return false
	}
	return math.Abs(a.Val-o.Val) <= eps
}

// Value implements driver.Valuer as JSON (prefer expanded columns in migrations).
func (m Measure) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan implements sql.Scanner.
func (m *Measure) Scan(value any) error {
	if m == nil {
		return fmt.Errorf("measure: Scan on nil *Measure")
	}
	if value == nil {
		*m = Measure{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, m)
	case string:
		return json.Unmarshal([]byte(v), m)
	default:
		return fmt.Errorf("measure: unsupported Scan type %T", value)
	}
}

// MarshalJSON / UnmarshalJSON use object form.
func (m Measure) MarshalJSON() ([]byte, error) {
	type alias Measure
	return json.Marshal(alias(m))
}

func (m *Measure) UnmarshalJSON(data []byte) error {
	type alias Measure
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	a.Unit = Unit(strings.TrimSpace(string(a.Unit)))
	*m = Measure(a)
	return nil
}
