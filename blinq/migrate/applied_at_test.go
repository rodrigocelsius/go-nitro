package migrate

import (
	"testing"
	"time"
)

func TestParseAppliedAt(t *testing.T) {
	want := time.Date(2026, 10, 14, 17, 47, 19, 226088917, time.UTC)
	cases := map[string]any{
		"time.Time":            want,
		"RFC3339Nano":          "2026-10-14T17:47:19.226088917Z",
		"time.String legacy":   "2026-10-14 17:47:19.226088917 +0000 UTC",
		"with monotonic":       "2026-10-14 17:47:19.226088917 +0000 UTC m=+0.012345",
		"mysql bytes":          []byte("2026-10-14 17:47:19.226088917"),
		"sqlite datetime(now)": "2026-10-14 17:47:19.226088917",
	}
	for name, raw := range cases {
		if got := parseAppliedAt(raw); !got.Equal(want) {
			t.Errorf("%s: got %v want %v", name, got, want)
		}
	}
	if got := parseAppliedAt("garbage"); !got.IsZero() {
		t.Errorf("garbage: got %v", got)
	}
	if got := parseAppliedAt(nil); !got.IsZero() {
		t.Errorf("nil: got %v", got)
	}
}
