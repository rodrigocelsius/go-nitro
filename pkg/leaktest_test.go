package nitro

import (
	"sync"
	"testing"
	"time"
)

func TestCheckLeaks_NoLeak(t *testing.T) {
	CheckLeaks(t)
	_ = GoroutineCount()
}

func TestCheckLeaks_WithGoroutine(t *testing.T) {
	CheckLeaks(t)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond)
		}()
	}
	wg.Wait()
}

func TestCheckLeaksWithTimeout(t *testing.T) {
	CheckLeaksWithTimeout(t, 50*time.Millisecond)
	_ = GoroutineCount()
}

func TestGoroutineCount(t *testing.T) {
	before := GoroutineCount()
	if before <= 0 {
		t.Errorf("GoroutineCount = %d, want > 0", before)
	}
}
