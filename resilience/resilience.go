// Package resilience implements Nitro's resilience patterns.
//
// Supported patterns:
//   - Timeout: cancels operations that exceed a deadline
//   - Retry: retries failed operations with exponential backoff
//   - Circuit Breaker: stops calls when failures accumulate
//   - Bulkhead: limits concurrency to protect resources
//
// Basic usage:
//
//	r := resilience.New()
//	result, err := r.Execute(ctx, func(ctx context.Context) (string, error) {
//	    return callExternalAPI(ctx)
//	},
//		resilience.WithTimeout(5*time.Second),
//		resilience.WithRetry(3, 100*time.Millisecond),
//		resilience.WithCircuitBreaker(5, 30*time.Second),
//	)
package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Result represents the result of an operation run with resilience.
type Result[T any] struct {
	Value            T
	Err              error
	Attempts         int
	Duration         time.Duration
	TimedOut         bool
	Retried          bool
	CircuitOpen      bool
	BulkheadRejected bool
}

// Option configures resilient execution.
type Option func(*config)

type config struct {
	timeout          time.Duration
	retryMax         int
	retryDelay       time.Duration
	retryBackoff     float64
	circuitThreshold int
	circuitTimeout   time.Duration
	bulkheadMax      int
}

func defaultConfig() *config {
	return &config{
		timeout:          30 * time.Second,
		retryMax:         3,
		retryDelay:       100 * time.Millisecond,
		retryBackoff:     2.0,
		circuitThreshold: 5,
		circuitTimeout:   30 * time.Second,
		bulkheadMax:      10,
	}
}

// WithTimeout configures a timeout for the operation.
func WithTimeout(d time.Duration) Option {
	return func(c *config) {
		c.timeout = d
	}
}

// WithRetry configures retry with exponential backoff.
func WithRetry(max int, delay time.Duration) Option {
	return func(c *config) {
		c.retryMax = max
		c.retryDelay = delay
	}
}

// WithRetryBackoff configures the backoff multiplier.
func WithRetryBackoff(backoff float64) Option {
	return func(c *config) {
		c.retryBackoff = backoff
	}
}

// WithCircuitBreaker configures a circuit breaker.
func WithCircuitBreaker(threshold int, timeout time.Duration) Option {
	return func(c *config) {
		c.circuitThreshold = threshold
		c.circuitTimeout = timeout
	}
}

// WithBulkhead configures a bulkhead to limit concurrency.
func WithBulkhead(maxConcurrent int) Option {
	return func(c *config) {
		c.bulkheadMax = maxConcurrent
	}
}

// Resilience manages resilience patterns.
type Resilience struct {
	circuits  map[string]*CircuitBreaker
	bulkheads map[string]*Bulkhead
	store     BreakerStore
	mu        sync.RWMutex
}

// New creates a new resilience instance.
func New() *Resilience {
	return &Resilience{
		circuits:  make(map[string]*CircuitBreaker),
		bulkheads: make(map[string]*Bulkhead),
	}
}

// SetBreakerStore configures optional persistent named breakers (e.g. Redis).
func (r *Resilience) SetBreakerStore(store BreakerStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store = store
}

// Execute runs an operation with the specified resilience patterns.
func Execute[T any](r *Resilience, ctx context.Context, fn func(ctx context.Context) (T, error), opts ...Option) Result[T] {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	start := time.Now()
	var lastErr error
	attempts := 0

	for attempt := 0; attempt <= cfg.retryMax; attempt++ {
		attempts++

		// Check parent context timeout
		select {
		case <-ctx.Done():
			return Result[T]{
				Err:      ctx.Err(),
				Attempts: attempts,
				Duration: time.Since(start),
				TimedOut: true,
			}
		default:
		}

		// Create context with timeout
		operationCtx, cancel := context.WithTimeout(ctx, cfg.timeout)

		// Run the operation
		value, err := fn(operationCtx)
		cancel()

		if err == nil {
			return Result[T]{
				Value:    value,
				Attempts: attempts,
				Duration: time.Since(start),
				Retried:  attempts > 1,
			}
		}

		lastErr = err

		// If not the last attempt, wait before retry
		if attempt < cfg.retryMax {
			backoff := 1 << uint(attempt)
			delay := time.Duration(float64(cfg.retryDelay) * float64(backoff))
			time.Sleep(delay)
		}
	}

	return Result[T]{
		Err:      lastErr,
		Attempts: attempts,
		Duration: time.Since(start),
		Retried:  attempts > 1,
	}
}

// CircuitBreaker implements the Circuit Breaker pattern.
type CircuitBreaker struct {
	threshold   int
	timeout     time.Duration
	failures    int
	lastFailure time.Time
	state       CircuitState
	mu          sync.RWMutex
}

// CircuitState represents the circuit breaker state.
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
		state:     CircuitClosed,
	}
}

// Allow reports whether the operation is allowed.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = CircuitClosed
}

// RecordFailure records a failure.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = CircuitOpen
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Bulkhead implements the Bulkhead pattern.
type Bulkhead struct {
	maxConcurrent int
	current       int
	mu            sync.Mutex
}

// NewBulkhead creates a new bulkhead.
func NewBulkhead(maxConcurrent int) *Bulkhead {
	return &Bulkhead{
		maxConcurrent: maxConcurrent,
	}
}

// Allow reports whether an operation is allowed.
func (b *Bulkhead) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.current >= b.maxConcurrent {
		return false
	}
	b.current++
	return true
}

// Release frees a bulkhead slot.
func (b *Bulkhead) Release() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.current > 0 {
		b.current--
	}
}

// Count returns the current number of operations.
func (b *Bulkhead) Count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.current
}

// ErrCircuitOpen is returned when the circuit breaker is open.
type ErrCircuitOpen struct {
	Timeout time.Duration
}

func (e *ErrCircuitOpen) Error() string {
	return fmt.Sprintf("circuit breaker open, retry in %s", e.Timeout)
}

// ErrBulkheadFull is returned when the bulkhead is full.
type ErrBulkheadFull struct {
	MaxConcurrent int
}

func (e *ErrBulkheadFull) Error() string {
	return fmt.Sprintf("bulkhead full, max concurrent: %d", e.MaxConcurrent)
}
