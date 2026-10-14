package temporal_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2/temporal"
)

func TestDateTime_JSONRoundTrip(t *testing.T) {
	dt := temporal.DateTimeFrom(time.Date(2026, 8, 5, 19, 30, 0, 0, time.UTC))
	b, err := json.Marshal(dt)
	if err != nil {
		t.Fatal(err)
	}
	var out temporal.DateTime
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !dt.Equal(out) {
		t.Fatalf("got %v want %v", out.Time(), dt.Time())
	}
}

func TestDate_ScanISO(t *testing.T) {
	var d temporal.Date
	if err := d.Scan("2026-08-05"); err != nil {
		t.Fatal(err)
	}
	if d.String() != "2026-08-05" {
		t.Fatal(d.String())
	}
}

func TestDateTime_ScanGoStringAndRFC3339(t *testing.T) {
	var d temporal.DateTime
	if err := d.Scan("2026-08-20 10:46:11.873016482 +0000 UTC"); err != nil {
		t.Fatal(err)
	}
	if d.IsZero() {
		t.Fatal("zero")
	}
	var d2 temporal.DateTime
	if err := d2.Scan("2026-08-20T10:46:11.873016482Z"); err != nil {
		t.Fatal(err)
	}
	v, err := d2.Value()
	if err != nil {
		t.Fatal(err)
	}
	tt, ok := v.(time.Time)
	if !ok || tt.IsZero() {
		t.Fatalf("Value want time.Time, got %#v", v)
	}
}

func TestCombineSplit(t *testing.T) {
	d := temporal.NewDate(2026, 8, 5)
	tod := temporal.NewTimeOfDay(19, 30, 0, 0)
	dt := temporal.CombineLocal(d, tod, time.UTC)
	d2, tod2 := temporal.SplitLocal(dt, time.UTC)
	if !d.Equal(d2) || tod.Hour() != tod2.Hour() {
		t.Fatalf("roundtrip fail: %v %v", d2, tod2)
	}
}

func TestFormatParseDate_ptBR(t *testing.T) {
	d := temporal.NewDate(2026, 8, 5)
	s := temporal.FormatDate(d, "pt-BR")
	if s != "05/08/2026" {
		t.Fatalf("got %q", s)
	}
	parsed, err := temporal.ParseDate(s, "pt-BR")
	if err != nil || !parsed.Equal(d) {
		t.Fatalf("parse: %v %v", parsed, err)
	}
}
