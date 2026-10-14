package fasthttp

import (
	"testing"
	"time"
)

func TestCachedLogTime_MatchesFormat(t *testing.T) {
	layout := "2006/01/02 15:04:05"
	got := cachedLogTime(layout)
	want := time.Now().Format(layout)
	if got != want {
		// a second boundary may have passed between the two calls
		if got2 := cachedLogTime(layout); got2 != time.Now().Format(layout) {
			t.Fatalf("got %q want %q", got2, want)
		}
	}
	if cachedLogTime("15:04") == got {
		t.Fatal("cache must be keyed by layout")
	}
}
