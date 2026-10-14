package nitro

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rodrigocelsius/go-nitro/resilience/v2"
)

// Resilience coordinates retry / timeout / circuit breaker / bulkhead.
type Resilience = resilience.Resilience

// ResilienceResult is the outcome of Execute / ExecuteNamed.
type ResilienceResult[T any] = resilience.Result[T]

// ResilienceOption configures Execute.
type ResilienceOption = resilience.Option

// CircuitBreaker is an in-memory breaker.
type CircuitBreaker = resilience.CircuitBreaker

// CircuitState is closed / open / half-open.
type CircuitState = resilience.CircuitState

// NamedBreaker is a named (optionally Redis-backed) circuit breaker.
type NamedBreaker = resilience.NamedBreaker

// BreakerStore persists named breaker state.
type BreakerStore = resilience.BreakerStore

// ErrCircuitOpen is returned when the circuit is open.
type ErrCircuitOpen = resilience.ErrCircuitOpen

// ErrBulkheadFull is returned when bulkhead rejects.
type ErrBulkheadFull = resilience.ErrBulkheadFull

const (
	CircuitClosed   = resilience.CircuitClosed
	CircuitOpen     = resilience.CircuitOpen
	CircuitHalfOpen = resilience.CircuitHalfOpen
)

// NewResilience creates a resilience coordinator.
func NewResilience() *Resilience {
	return resilience.New()
}

// SetBreakerStore configures persistent named circuit breakers (e.g. Redis).
func SetBreakerStore(r *Resilience, store BreakerStore) {
	r.SetBreakerStore(store)
}

// WithTimeout sets per-attempt timeout.
func WithTimeout(d time.Duration) ResilienceOption {
	return resilience.WithTimeout(d)
}

// WithRetry sets retry count and base delay.
func WithRetry(max int, delay time.Duration) ResilienceOption {
	return resilience.WithRetry(max, delay)
}

// WithRetryBackoff sets exponential backoff multiplier.
func WithRetryBackoff(backoff float64) ResilienceOption {
	return resilience.WithRetryBackoff(backoff)
}

// WithCircuitBreaker configures failure threshold and open timeout.
func WithCircuitBreaker(threshold int, timeout time.Duration) ResilienceOption {
	return resilience.WithCircuitBreaker(threshold, timeout)
}

// WithBulkhead limits concurrency.
func WithBulkhead(maxConcurrent int) ResilienceOption {
	return resilience.WithBulkhead(maxConcurrent)
}

// Execute runs fn with resilience options.
func Execute[T any](r *Resilience, ctx context.Context, fn func(ctx context.Context) (T, error), opts ...ResilienceOption) ResilienceResult[T] {
	return resilience.Execute(r, ctx, fn, opts...)
}

// ExecuteNamed runs fn behind a named circuit breaker (e.g. "tenant:UF").
func ExecuteNamed[T any](r *Resilience, ctx context.Context, name string, fn func(ctx context.Context) (T, error), opts ...ResilienceOption) ResilienceResult[T] {
	return resilience.ExecuteNamed(r, ctx, name, fn, opts...)
}

// NewMemoryBreakerStore returns an in-process breaker store.
func NewMemoryBreakerStore() BreakerStore {
	return resilience.NewMemoryBreakerStore()
}

type redisCodec struct {
	c *redis.Client
}

func (r redisCodec) Get(ctx context.Context, key string) (string, error) {
	return r.c.Get(ctx, key).Result()
}

func (r redisCodec) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.c.Set(ctx, key, value, ttl).Err()
}

// NewRedisBreakerStore persists named breakers in Redis.
func NewRedisBreakerStore(client *redis.Client, keyPrefix string, ttl time.Duration) BreakerStore {
	return resilience.NewRedisBreakerStore(redisCodec{c: client}, keyPrefix, ttl)
}
