package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/resilience/v2"
)

func TestNamedBreakerOpens(t *testing.T) {
	r := resilience.New()
	nb := r.Named("tenant:SP", 2, time.Minute, nil)
	ctx := context.Background()
	if !nb.Allow(ctx) {
		t.Fatal("expected allow")
	}
	nb.RecordFailure(ctx)
	nb.RecordFailure(ctx)
	if nb.Allow(ctx) {
		t.Fatal("expected open")
	}
	if nb.State(ctx) != resilience.CircuitOpen {
		t.Fatalf("state=%v", nb.State(ctx))
	}
}

func TestExecuteNamedSuccess(t *testing.T) {
	r := resilience.New()
	res := resilience.ExecuteNamed(r, context.Background(), "x", func(ctx context.Context) (int, error) {
		return 42, nil
	}, resilience.WithTimeout(time.Second), resilience.WithCircuitBreaker(3, time.Minute))
	if res.Err != nil || res.Value != 42 {
		t.Fatalf("%+v", res)
	}
}

func TestExecuteNamedCircuit(t *testing.T) {
	r := resilience.New()
	boom := errors.New("boom")
	for i := 0; i < 3; i++ {
		_ = resilience.ExecuteNamed(r, context.Background(), "y", func(ctx context.Context) (int, error) {
			return 0, boom
		}, resilience.WithTimeout(time.Second), resilience.WithRetry(0, 0), resilience.WithCircuitBreaker(3, time.Minute))
	}
	res := resilience.ExecuteNamed(r, context.Background(), "y", func(ctx context.Context) (int, error) {
		return 1, nil
	}, resilience.WithTimeout(time.Second), resilience.WithCircuitBreaker(3, time.Minute))
	if !res.CircuitOpen {
		t.Fatalf("expected circuit open, got %+v", res)
	}
}

func TestMemoryBreakerStore(t *testing.T) {
	store := resilience.NewMemoryBreakerStore()
	ctx := context.Background()
	_ = store.Set(ctx, "a", 2, time.Now(), resilience.CircuitOpen)
	f, _, st, ok, err := store.Get(ctx, "a")
	if err != nil || !ok || f != 2 || st != resilience.CircuitOpen {
		t.Fatalf("f=%v st=%v ok=%v err=%v", f, st, ok, err)
	}
}
