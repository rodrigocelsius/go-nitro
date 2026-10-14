package jwt

import (
	"context"
	"sync"
	"time"
)

// RevocationStore persists revoked token JTIs.
//
// The default (memoryStore) keeps revocations in-process: they are lost on
// restart and invisible to other replicas — a token revoked on one
// instance behind a load balancer stays valid on every other instance
// until it expires naturally. This is acceptable only for
// single-instance/dev setups; it is a blocking limitation for any
// multi-instance/multi-replica production deployment (see vuln.md V-26).
//
// For production behind a load balancer, plug a shared store instead.
// A ready-to-use Redis-backed implementation is provided in
// github.com/rodrigocelsius/go-nitro/adapters/auth/jwt/redisrevocation/v2:
//
//	import "github.com/rodrigocelsius/go-nitro/adapters/auth/jwt/redisrevocation/v2"
//
//	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
//	p.SetRevocationStore(redisrevocation.New(rdb))
//
// Or implement RevocationStore against any other shared store you
// already run (Redis, Memcached, a DB table, ...).
type RevocationStore interface {
	// Revoke marks jti as revoked until exp. Implementations should treat
	// the entry as disposable after exp.
	Revoke(ctx context.Context, jti string, exp time.Time) error
	// Revoked reports whether jti is currently revoked.
	Revoked(ctx context.Context, jti string) (bool, error)
}

// memoryStore is the default in-process RevocationStore.
type memoryStore struct {
	mu      sync.Mutex
	revoked map[string]time.Time // jti -> expiry
}

func newMemoryStore() *memoryStore {
	return &memoryStore{revoked: make(map[string]time.Time)}
}

func (m *memoryStore) Revoke(_ context.Context, jti string, exp time.Time) error {
	if jti == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, v := range m.revoked {
		if v.Before(now) {
			delete(m.revoked, k)
		}
	}
	m.revoked[jti] = exp
	return nil
}

func (m *memoryStore) Revoked(_ context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.revoked[jti]
	if !ok {
		return false, nil
	}
	if exp.Before(time.Now()) {
		delete(m.revoked, jti)
		return false, nil
	}
	return true, nil
}

// SetRevocationStore replaces where revoked JTIs are persisted.
// Pass nil to restore the in-memory default. Safe for concurrent use:
// swaps happen atomically via the provider's store mutex.
func (p *Provider) SetRevocationStore(s RevocationStore) {
	p.storeMu.Lock()
	defer p.storeMu.Unlock()
	if s == nil {
		p.store = newMemoryStore()
		return
	}
	p.store = s
}
