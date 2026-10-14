package geo

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const earthRadiusM = 6371000.0 // mean Earth radius (meters)

// ParseLatLng parses "lat,lng" (whitespace tolerated).
func ParseLatLng(s string) (LatLng, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return LatLng{}, ErrInvalidFormat
	}
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return LatLng{}, ErrInvalidFormat
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return LatLng{}, ErrInvalidFormat
	}
	lng, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return LatLng{}, ErrInvalidFormat
	}
	return NewLatLng(lat, lng)
}

// ParseLatitude parses a decimal latitude string.
func ParseLatitude(s string) (Latitude, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, ErrInvalidFormat
	}
	return NewLatitude(f)
}

// ParseLongitude parses a decimal longitude string.
func ParseLongitude(s string) (Longitude, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, ErrInvalidFormat
	}
	return NewLongitude(f)
}

// DistanceMeters returns great-circle distance (Haversine) in meters.
func DistanceMeters(a, b LatLng) float64 {
	φ1 := float64(a.Lat) * math.Pi / 180
	φ2 := float64(b.Lat) * math.Pi / 180
	Δφ := (float64(b.Lat) - float64(a.Lat)) * math.Pi / 180
	Δλ := (float64(b.Lng) - float64(a.Lng)) * math.Pi / 180
	sinΔφ := math.Sin(Δφ / 2)
	sinΔλ := math.Sin(Δλ / 2)
	h := sinΔφ*sinΔφ + math.Cos(φ1)*math.Cos(φ2)*sinΔλ*sinΔλ
	return 2 * earthRadiusM * math.Asin(math.Min(1, math.Sqrt(h)))
}

// FormatLatLng formats for display.
func FormatLatLng(p LatLng, locale string) string {
	locale = strings.ToLower(strings.ReplaceAll(locale, "_", "-"))
	if strings.HasPrefix(locale, "pt") || strings.HasPrefix(locale, "es") || strings.HasPrefix(locale, "fr") {
		return formatDMS(p)
	}
	return formatCoord(float64(p.Lat)) + ", " + formatCoord(float64(p.Lng))
}

func formatDMS(p LatLng) string {
	latH, lngH := "N", "E"
	lat := float64(p.Lat)
	lng := float64(p.Lng)
	if lat < 0 {
		lat = -lat
		latH = "S"
	}
	if lng < 0 {
		lng = -lng
		lngH = "O"
	}
	return fmt.Sprintf("%s %s, %s %s", toDMS(lat), latH, toDMS(lng), lngH)
}

func toDMS(absDeg float64) string {
	d := int(absDeg)
	mFloat := (absDeg - float64(d)) * 60
	m := int(mFloat)
	s := (mFloat - float64(m)) * 60
	return fmt.Sprintf("%d°%02d′%05.2f″", d, m, s)
}
