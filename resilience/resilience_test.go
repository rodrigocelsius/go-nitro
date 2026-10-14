package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestExecute_Success(t *testing.T) {
	r := New()

	result := Execute(r, context.Background(), func(ctx context.Context) (string, error) {
		return "success", nil
	})

	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}

	if result.Value != "success" {
		t.Errorf("expected 'success', got %q", result.Value)
	}

	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", result.Attempts)
	}
}

func TestExecute_Retry(t *testing.T) {
	r := New()

	var attempts int32
	result := Execute(r, context.Background(), func(ctx context.Context) (string, error) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			return "", errors.New("temporary error")
		}
		return "success", nil
	}, WithRetry(3, 10*time.Millisecond))

	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}

	if result.Value != "success" {
		t.Errorf("expected 'success', got %q", result.Value)
	}

	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", result.Attempts)
	}

	if !result.Retried {
		t.Error("expected Retried to be true")
	}
}

func TestExecute_RetryExhausted(t *testing.T) {
	r := New()

	result := Execute(r, context.Background(), func(ctx context.Context) (string, error) {
		return "", errors.New("permanent error")
	}, WithRetry(2, 10*time.Millisecond))

	if result.Err == nil {
		t.Error("expected error, got nil")
	}

	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts (1 + 2 retries), got %d", result.Attempts)
	}
}

func TestExecute_Timeout(t *testing.T) {
	r := New()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result := Execute(r, ctx, func(ctx context.Context) (string, error) {
		select {
		case <-time.After(200 * time.Millisecond):
			return "success", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}, WithTimeout(100*time.Millisecond))

	if result.Err == nil {
		t.Error("expected timeout error, got nil")
	}

	if !result.TimedOut {
		t.Error("expected TimedOut to be true")
	}
}

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// Initially closed
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed, got %v", cb.State())
	}

	// Record failures
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Should be open
	if cb.State() != CircuitOpen {
		t.Errorf("expected open, got %v", cb.State())
	}

	// Must not allow operations
	if cb.Allow() {
		t.Error("expected Allow to return false when open")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Should allow operations (half-open)
	if !cb.Allow() {
		t.Error("expected Allow to return true when half-open")
	}

	// Success closes the circuit
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after success, got %v", cb.State())
	}
}

func TestBulkhead(t *testing.T) {
	b := NewBulkhead(2)

	// Should allow up to the limit
	if !b.Allow() {
		t.Error("expected Allow to return true")
	}
	if !b.Allow() {
		t.Error("expected Allow to return true")
	}

	// Should reject when full
	if b.Allow() {
		t.Error("expected Allow to return false when full")
	}

	// Release a slot
	b.Release()

	// Should allow again
	if !b.Allow() {
		t.Error("expected Allow to return true after release")
	}

	// Check count
	if b.Count() != 2 {
		t.Errorf("expected count 2, got %d", b.Count())
	}
}

func TestExecute_AllOptions(t *testing.T) {
	r := New()

	var attempts int32
	result := Execute(r, context.Background(), func(ctx context.Context) (int, error) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 2 {
			return 0, errors.New("temporary")
		}
		return 42, nil
	},
		WithTimeout(5*time.Second),
		WithRetry(3, 10*time.Millisecond),
		WithRetryBackoff(1.5),
	)

	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}

	if result.Value != 42 {
		t.Errorf("expected 42, got %d", result.Value)
	}

	if result.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", result.Attempts)
	}
}
