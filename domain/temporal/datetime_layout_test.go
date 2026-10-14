package temporal

import (
	"testing"
	"time"
)

func TestDateTimeParseShapes(t *testing.T) {
	want := time.Date(2026, 10, 14, 17, 47, 19, 226088917, time.UTC)
	for _, s := range []string{
		"2026-10-14T17:47:19.226088917Z",
		"2026-10-14 17:47:19.226088917 +0000 UTC",
		"2026-10-14 17:47:19.226088917+00:00",
		"2026-10-14 17:47:19.226088917",
		"2026-10-14T17:47:19.226088917",
	} {
		var d DateTime
		if err := d.Scan(s); err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		if !d.Time().Equal(want) {
			t.Errorf("%q → %v", s, d.Time())
		}
	}
	for _, s := range []string{"2026-10-14T17:47:19Z", "2026-10-14 17:47:19"} {
		var d DateTime
		if err := d.Scan(s); err != nil {
			t.Errorf("%q: %v", s, err)
		}
	}
	var d DateTime
	if d.Scan("garbage") == nil {
		t.Error("garbage must fail")
	}
}

func BenchmarkDateTimeScanSQLite(b *testing.B) {
	b.ReportAllocs()
	var d DateTime
	for i := 0; i < b.N; i++ {
		_ = d.Scan("2026-10-14 17:47:19.226088917 +0000 UTC")
	}
}
