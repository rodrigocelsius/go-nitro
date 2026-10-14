package nitro

import (
	"runtime"
	"testing"
	"time"
)

// CheckLeaks registers a goroutine leak check at the end of the test.
// If the goroutine count increases after the test, the test fails.
//
// Usage:
//
//	func TestMyFeature(t *testing.T) {
//	    pkg.CheckLeaks(t)
//	    // ... code that must not leak goroutines
//	}
func CheckLeaks(t *testing.T) {
	t.Helper()
	before := runtime.NumGoroutine()

	t.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
		after := runtime.NumGoroutine()
		if after > before {
			t.Errorf("goroutine leak detected: before=%d, after=%d (leaked %d)",
				before, after, after-before)
		}
	})
}

// CheckLeaksWithTimeout allows a custom timeout.
func CheckLeaksWithTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()
	before := runtime.NumGoroutine()

	t.Cleanup(func() {
		time.Sleep(timeout)
		after := runtime.NumGoroutine()
		if after > before {
			t.Errorf("goroutine leak detected: before=%d, after=%d (leaked %d)",
				before, after, after-before)
		}
	})
}

// GoroutineCount returns the current number of goroutines.
// Useful for assertions in tests.
func GoroutineCount() int {
	return runtime.NumGoroutine()
}
