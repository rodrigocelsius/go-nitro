package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

const defaultRedisKeyPrefix = "nitro:cache:"

// RedisCache implements domain.Cache on Redis (L2).
type RedisCache struct {
	client     *redis.Client
	keyPrefix  string
	defaultTTL time.Duration
}

// RedisOption configures RedisCache.
type RedisOption func(*RedisCache)

// WithKeyPrefix sets the key prefix (default "nitro:cache:").
func WithKeyPrefix(prefix string) RedisOption {
	return func(c *RedisCache) {
		if prefix != "" {
			c.keyPrefix = prefix
		}
	}
}

// WithDefaultTTL sets the TTL used when Set receives ttl == 0.
func WithDefaultTTL(ttl time.Duration) RedisOption {
	return func(c *RedisCache) {
		c.defaultTTL = ttl
	}
}

// NewRedisCache creates an L2 Redis cache.
func NewRedisCache(client *redis.Client, opts ...RedisOption) *RedisCache {
	c := &RedisCache{
		client:     client,
		keyPrefix:  defaultRedisKeyPrefix,
		defaultTTL: 5 * time.Minute,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *RedisCache) key(k string) string {
	return c.keyPrefix + k
}

// Get retrieves and unmarshals JSON into dest.
func (c *RedisCache) Get(key string, dest any) error {
	ctx := context.Background()
	data, err := c.client.Get(ctx, c.key(key)).Bytes()
	if err == redis.Nil {
		return domain.ErrCacheMiss{}
	}
	if err != nil {
		return fmt.Errorf("redis cache get: %w", err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("redis cache unmarshal: %w", err)
	}
	return nil
}

// Set marshals value as JSON and writes it with TTL.
func (c *RedisCache) Set(key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("redis cache marshal: %w", err)
	}
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	ctx := context.Background()
	if err := c.client.Set(ctx, c.key(key), data, ttl).Err(); err != nil {
		return fmt.Errorf("redis cache set: %w", err)
	}
	return nil
}

// Delete removes a key.
func (c *RedisCache) Delete(key string) error {
	ctx := context.Background()
	if err := c.client.Del(ctx, c.key(key)).Err(); err != nil {
		return fmt.Errorf("redis cache delete: %w", err)
	}
	return nil
}

// Invalidate removes keys matching pattern (Redis glob, relative to the prefix).
// Examples: "users:list:*", "events:*".
func (c *RedisCache) Invalidate(pattern string) error {
	if pattern == "" {
		return nil
	}
	ctx := context.Background()
	full := c.keyPrefix + pattern
	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, full, 100).Result()
		if err != nil {
			return fmt.Errorf("redis cache scan: %w", err)
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("redis cache invalidate del: %w", err)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

// Clear removes all keys with the cache prefix.
func (c *RedisCache) Clear() error {
	return c.Invalidate("*")
}

// Ping checks Redis connectivity.
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Client exposes the underlying Redis client (advanced / admin ops).
func (c *RedisCache) Client() *redis.Client {
	return c.client
}
