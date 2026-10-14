package geo

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// MaxFracDigits is the canonical decimal scale for WGS84 coords (~0.11 mm).
const MaxFracDigits = 9

var (
	ErrInvalidLatitude  = fmt.Errorf("geo: latitude out of range [-90, 90]")
	ErrInvalidLongitude = fmt.Errorf("geo: longitude out of range [-180, 180]")
	ErrInvalidFormat    = fmt.Errorf("geo: invalid format")
)

// Latitude is WGS84 latitude in degrees (float64, max 9 fractional digits).
type Latitude float64

// Longitude is WGS84 longitude in degrees (float64, max 9 fractional digits).
type Longitude float64

// LatLng is a geographic point.
type LatLng struct {
	Lat Latitude  `json:"lat"`
	Lng Longitude `json:"lng"`
}

// NewLatitude validates and rounds.
func NewLatitude(v float64) (Latitude, error) {
	if v < -90 || v > 90 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, ErrInvalidLatitude
	}
	return Latitude(roundFrac(v, MaxFracDigits)), nil
}

// NewLongitude validates and rounds.
func NewLongitude(v float64) (Longitude, error) {
	if v < -180 || v > 180 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, ErrInvalidLongitude
	}
	return Longitude(roundFrac(v, MaxFracDigits)), nil
}

// NewLatLng builds a validated point.
func NewLatLng(lat, lng float64) (LatLng, error) {
	la, err := NewLatitude(lat)
	if err != nil {
		return LatLng{}, err
	}
	lo, err := NewLongitude(lng)
	if err != nil {
		return LatLng{}, err
	}
	return LatLng{Lat: la, Lng: lo}, nil
}

// MustLatLng panics on invalid coords.
func MustLatLng(lat, lng float64) LatLng {
	p, err := NewLatLng(lat, lng)
	if err != nil {
		panic(err)
	}
	return p
}

func (lat Latitude) Float64() float64  { return float64(lat) }
func (lng Longitude) Float64() float64 { return float64(lng) }

func (lat Latitude) Valid() bool {
	v := float64(lat)
	return v >= -90 && v <= 90 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func (lng Longitude) Valid() bool {
	v := float64(lng)
	return v >= -180 && v <= 180 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func (lat Latitude) IsZero() bool  { return lat == 0 }
func (lng Longitude) IsZero() bool { return lng == 0 }

func (p LatLng) IsZero() bool { return p.Lat == 0 && p.Lng == 0 }

func (p LatLng) Valid() bool { return p.Lat.Valid() && p.Lng.Valid() }

// Round returns coords rounded to MaxFracDigits.
func (lat Latitude) Round() Latitude   { return Latitude(roundFrac(float64(lat), MaxFracDigits)) }
func (lng Longitude) Round() Longitude { return Longitude(roundFrac(float64(lng), MaxFracDigits)) }
func (p LatLng) Round() LatLng         { return LatLng{Lat: p.Lat.Round(), Lng: p.Lng.Round()} }

func (lat Latitude) String() string  { return formatCoord(float64(lat)) }
func (lng Longitude) String() string { return formatCoord(float64(lng)) }
func (p LatLng) String() string      { return p.Lat.String() + "," + p.Lng.String() }

func formatCoord(v float64) string {
	s := strconv.FormatFloat(roundFrac(v, MaxFracDigits), 'f', MaxFracDigits, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func roundFrac(v float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	return math.Round(v*p) / p
}

// Value implements driver.Valuer — decimal string for max fidelity (NUMERIC/TEXT).
func (lat Latitude) Value() (driver.Value, error) {
	if !lat.Valid() {
		return nil, ErrInvalidLatitude
	}
	return formatCoord(float64(lat)), nil
}

func (lng Longitude) Value() (driver.Value, error) {
	if !lng.Valid() {
		return nil, ErrInvalidLongitude
	}
	return formatCoord(float64(lng)), nil
}

func (p LatLng) Value() (driver.Value, error) {
	if p.IsZero() {
		return nil, nil
	}
	if !p.Valid() {
		return nil, ErrInvalidFormat
	}
	return json.Marshal(p.Round())
}

// Scan implements sql.Scanner.
func (lat *Latitude) Scan(value any) error {
	if lat == nil {
		return fmt.Errorf("geo: Scan on nil *Latitude")
	}
	f, err := scanFloat(value)
	if err != nil {
		return err
	}
	if f == nil {
		*lat = 0
		return nil
	}
	v, err := NewLatitude(*f)
	if err != nil {
		return err
	}
	*lat = v
	return nil
}

func (lng *Longitude) Scan(value any) error {
	if lng == nil {
		return fmt.Errorf("geo: Scan on nil *Longitude")
	}
	f, err := scanFloat(value)
	if err != nil {
		return err
	}
	if f == nil {
		*lng = 0
		return nil
	}
	v, err := NewLongitude(*f)
	if err != nil {
		return err
	}
	*lng = v
	return nil
}

func (p *LatLng) Scan(value any) error {
	if p == nil {
		return fmt.Errorf("geo: Scan on nil *LatLng")
	}
	if value == nil {
		*p = LatLng{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, p)
	case string:
		if strings.Contains(v, "{") {
			return json.Unmarshal([]byte(v), p)
		}
		parsed, err := ParseLatLng(v)
		if err != nil {
			return err
		}
		*p = parsed
		return nil
	default:
		return fmt.Errorf("geo: unsupported LatLng Scan type %T", value)
	}
}

func scanFloat(value any) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case float64:
		return &v, nil
	case float32:
		f := float64(v)
		return &f, nil
	case int64:
		f := float64(v)
		return &f, nil
	case int:
		f := float64(v)
		return &f, nil
	case []byte:
		s := strings.TrimSpace(string(v))
		if s == "" {
			return nil, nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, err
		}
		return &f, nil
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil, nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, err
		}
		return &f, nil
	default:
		return nil, fmt.Errorf("geo: unsupported Scan type %T", value)
	}
}

func (lat Latitude) MarshalJSON() ([]byte, error) {
	return json.Marshal(roundFrac(float64(lat), MaxFracDigits))
}

func (lat *Latitude) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*lat = 0
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	v, err := NewLatitude(f)
	if err != nil {
		return err
	}
	*lat = v
	return nil
}

func (lng Longitude) MarshalJSON() ([]byte, error) {
	return json.Marshal(roundFrac(float64(lng), MaxFracDigits))
}

func (lng *Longitude) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*lng = 0
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	v, err := NewLongitude(f)
	if err != nil {
		return err
	}
	*lng = v
	return nil
}
