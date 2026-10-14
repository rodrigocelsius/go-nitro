package resilience

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// BreakerStore persists named circuit breaker state (optional Redis / memory).
type BreakerStore interface {
	Get(ctx context.Context, name string) (failures int, lastFailure time.Time, state CircuitState, ok bool, err error)
	Set(ctx context.Context, name string, failures int, lastFailure time.Time, state CircuitState) error
}

type memoryBreakerStore struct {
	mu   sync.RWMutex
	data map[string]storedBreaker
}

type storedBreaker struct {
	Failures    int          `json:"failures"`
	LastFailure time.Time    `json:"last_failure"`
	State       CircuitState `json:"state"`
}

// NewMemoryBreakerStore returns an in-process BreakerStore.
func NewMemoryBreakerStore() BreakerStore {
	return &memoryBreakerStore{data: make(map[string]storedBreaker)}
}

func (s *memoryBreakerStore) Get(_ context.Context, name string) (int, time.Time, CircuitState, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[name]
	if !ok {
		return 0, time.Time{}, CircuitClosed, false, nil
	}
	return v.Failures, v.LastFailure, v.State, true, nil
}

func (s *memoryBreakerStore) Set(_ context.Context, name string, failures int, lastFailure time.Time, state CircuitState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[name] = storedBreaker{Failures: failures, LastFailure: lastFailure, State: state}
	return nil
}

// RedisBreakerCodec is implemented by callers wrapping go-redis (keeps resilience free of redis dep).
type RedisBreakerCodec interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

type redisBreakerStore struct {
	client RedisBreakerCodec
	prefix string
	ttl    time.Duration
}

// NewRedisBreakerStore wraps a simple Get/Set client (e.g. Redis) for named breakers.
func NewRedisBreakerStore(client RedisBreakerCodec, keyPrefix string, ttl time.Duration) BreakerStore {
	if keyPrefix == "" {
		keyPrefix = "nitro:cb:"
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &redisBreakerStore{client: client, prefix: keyPrefix, ttl: ttl}
}

func (s *redisBreakerStore) Get(ctx context.Context, name string) (int, time.Time, CircuitState, bool, error) {
	raw, err := s.client.Get(ctx, s.prefix+name)
	if err != nil || raw == "" {
		return 0, time.Time{}, CircuitClosed, false, nil
	}
	var v storedBreaker
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return 0, time.Time{}, CircuitClosed, false, err
	}
	return v.Failures, v.LastFailure, v.State, true, nil
}

func (s *redisBreakerStore) Set(ctx context.Context, name string, failures int, lastFailure time.Time, state CircuitState) error {
	b, err := json.Marshal(storedBreaker{Failures: failures, LastFailure: lastFailure, State: state})
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.prefix+name, string(b), s.ttl)
}

// NamedBreaker is a circuit breaker addressable by name (e.g. "tenant:UF").
type NamedBreaker struct {
	name      string
	threshold int
	timeout   time.Duration
	store     BreakerStore
	local     *CircuitBreaker
}

// Breaker returns (or creates) a named in-memory circuit breaker on Resilience.
func (r *Resilience) Breaker(name string, threshold int, timeout time.Duration) *CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cb, ok := r.circuits[name]; ok {
		return cb
	}
	if threshold <= 0 {
		threshold = 5
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cb := NewCircuitBreaker(threshold, timeout)
	r.circuits[name] = cb
	return cb
}

// Named creates a NamedBreaker. If store is nil, uses Resilience.SetBreakerStore (or local-only).
func (r *Resilience) Named(name string, threshold int, timeout time.Duration, store BreakerStore) *NamedBreaker {
	if store == nil {
		r.mu.RLock()
		store = r.store
		r.mu.RUnlock()
	}
	if threshold <= 0 {
		threshold = 5
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &NamedBreaker{
		name:      name,
		threshold: threshold,
		timeout:   timeout,
		store:     store,
		local:     r.Breaker(name, threshold, timeout),
	}
}

// Allow reports whether calls may proceed.
func (nb *NamedBreaker) Allow(ctx context.Context) bool {
	if nb.store != nil {
		if failures, last, state, ok, err := nb.store.Get(ctx, nb.name); err == nil && ok {
			nb.local.mu.Lock()
			nb.local.failures = failures
			nb.local.lastFailure = last
			nb.local.state = state
			nb.local.mu.Unlock()
		}
	}
	return nb.local.Allow()
}

// RecordSuccess resets the breaker.
func (nb *NamedBreaker) RecordSuccess(ctx context.Context) {
	nb.local.RecordSuccess()
	_ = nb.persist(ctx)
}

// RecordFailure increments failures and may open the circuit.
func (nb *NamedBreaker) RecordFailure(ctx context.Context) {
	nb.local.RecordFailure()
	_ = nb.persist(ctx)
}

// State returns the current circuit state.
func (nb *NamedBreaker) State(ctx context.Context) CircuitState {
	_ = nb.Allow(ctx)
	return nb.local.State()
}

func (nb *NamedBreaker) persist(ctx context.Context) error {
	if nb.store == nil {
		return nil
	}
	nb.local.mu.RLock()
	failures := nb.local.failures
	last := nb.local.lastFailure
	state := nb.local.state
	nb.local.mu.RUnlock()
	if err := nb.store.Set(ctx, nb.name, failures, last, state); err != nil {
		return fmt.Errorf("resilience: persist breaker %s: %w", nb.name, err)
	}
	return nil
}

// ExecuteNamed runs fn guarded by a named circuit breaker (+ optional retry/timeout from opts).
func ExecuteNamed[T any](r *Resilience, ctx context.Context, name string, fn func(ctx context.Context) (T, error), opts ...Option) Result[T] {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	nb := r.Named(name, cfg.circuitThreshold, cfg.circuitTimeout, nil)
	if !nb.Allow(ctx) {
		var zero T
		return Result[T]{
			Value:       zero,
			Err:         &ErrCircuitOpen{Timeout: cfg.circuitTimeout},
			CircuitOpen: true,
		}
	}
	res := Execute(r, ctx, fn, opts...)
	if res.Err != nil {
		nb.RecordFailure(ctx)
	} else {
		nb.RecordSuccess(ctx)
	}
	return res
}
