// Package redisrevocation provides a ready-to-use Redis-backed
// jwt.RevocationStore, so multi-instance deployments don't have to
// hand-roll one to get out of the single-instance-only default
// (see adapters/auth/jwt/store.go and vuln.md V-26: the built-in
// in-process store keeps revoked JTIs in memory only, so a token
// revoked on one replica stays valid on every other replica behind the
// same load balancer until it naturally expires).
package redisrevocation

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store is a Redis-backed implementation of jwt.RevocationStore.
// Revoked JTIs are stored as keys with a TTL matching the token's
// remaining lifetime, so Redis itself reclaims them — no separate
// cleanup pass is needed, unlike the in-process default.
type Store struct {
	client    *redis.Client
	keyPrefix string
}

// New creates a Redis-backed RevocationStore.
//
//	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
//	provider.SetRevocationStore(redisrevocation.New(rdb))
func New(client *redis.Client) *Store {
	return &Store{client: client, keyPrefix: "nitro:jwt:revoked:"}
}

func (s *Store) key(jti string) string {
	return s.keyPrefix + jti
}

// Revoke marks jti as revoked until exp, using a Redis TTL so the key
// disappears on its own once the token would have expired anyway.
func (s *Store) Revoke(ctx context.Context, jti string, exp time.Time) error {
	if jti == "" {
		return nil
	}
	ttl := time.Until(exp)
	if ttl <= 0 {
		// Already expired: no point revoking, but not an error either.
		return nil
	}
	return s.client.Set(ctx, s.key(jti), "1", ttl).Err()
}

// Revoked reports whether jti is currently revoked.
func (s *Store) Revoked(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	n, err := s.client.Exists(ctx, s.key(jti)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
