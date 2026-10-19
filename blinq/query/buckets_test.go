package query

import (
	"testing"
	"time"
)

func TestDayBuckets(t *testing.T) {
	now := time.Date(2026, 3, 1, 15, 4, 0, 0, time.UTC)
	b := DayBuckets(3, now)
	if len(b) != 3 || b[0].Label != "2026-02-27" || b[2].Label != "2026-03-01" {
		t.Fatalf("buckets = %+v", b)
	}
	if !b[2].End.Equal(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("last end = %v", b[2].End)
	}
	if !b[0].End.Equal(b[1].Start) {
		t.Fatal("buckets must be contiguous")
	}
}

func TestHourBuckets(t *testing.T) {
	now := time.Date(2026, 3, 1, 15, 4, 0, 0, time.UTC)
	b := HourBuckets(2, now)
	if len(b) != 2 || !b[1].Start.Equal(time.Date(2026, 3, 1, 15, 0, 0, 0, time.UTC)) {
		t.Fatalf("buckets = %+v", b)
	}
}
