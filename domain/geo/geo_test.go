package geo_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2/geo"
)

func TestNewLatLng_Valid(t *testing.T) {
	p, err := geo.NewLatLng(-23.5505199, -46.6333094)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Valid() {
		t.Fatal("expected valid")
	}
}

func TestNewLatitude_OutOfRange(t *testing.T) {
	if _, err := geo.NewLatitude(91); err != geo.ErrInvalidLatitude {
		t.Fatalf("got %v", err)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	p := geo.MustLatLng(-23.55, -46.63)
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var out geo.LatLng
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Lat != p.Lat || out.Lng != p.Lng {
		t.Fatalf("%v vs %v", out, p)
	}
}

func TestValueScanString(t *testing.T) {
	lat := geo.Latitude(-23.5505199).Round()
	v, err := lat.Value()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("want string, got %T", v)
	}
	var lat2 geo.Latitude
	if err := lat2.Scan(s); err != nil {
		t.Fatal(err)
	}
	if math.Abs(lat2.Float64()-lat.Float64()) > 1e-9 {
		t.Fatalf("%v vs %v", lat2, lat)
	}
}

func TestHaversineSP_RJ(t *testing.T) {
	sp := geo.MustLatLng(-23.5505, -46.6333)
	rj := geo.MustLatLng(-22.9068, -43.1729)
	d := geo.DistanceMeters(sp, rj)
	if d < 350_000 || d > 370_000 {
		t.Fatalf("distance %.0f m", d)
	}
}

func TestParseLatLng(t *testing.T) {
	p, err := geo.ParseLatLng(" -23.55 , -46.63 ")
	if err != nil {
		t.Fatal(err)
	}
	if p.Lat != geo.Latitude(-23.55) {
		t.Fatal(p)
	}
}
