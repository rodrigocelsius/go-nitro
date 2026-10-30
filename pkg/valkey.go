package nitro

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

// Valkey is the BSD-3 licensed fork of Redis 7.2, kept by the Linux Foundation
// after Redis moved to RSAL/SSPL (7.4) and AGPL (8). It speaks the same RESP
// protocol, so it is served by the go-redis client Nitro already uses: every
// Redis adapter (RedisCache, RedisSessionStore, NewRedisBreakerStore, JWT
// revocation) works with a Valkey client unchanged.
//
//	if rdb, err := nitro.ValkeyClientFromEnv(); err != nil {
//	    log.Fatal(err)
//	} else if rdb != nil {
//	    app.SetCache(nitro.ValkeyCache(rdb))
//	}

// ValkeyConfig configures a Valkey connection. Each variable falls back to its
// REDIS_* name, so switching a deployment from Redis to Valkey needs no env change.
type ValkeyConfig struct {
	// Addr is host:port. Empty means "no Valkey" for ValkeyClientFromEnv.
	Addr     string `env:"VALKEY_ADDR,REDIS_ADDR"`
	Username string `env:"VALKEY_USERNAME,REDIS_USERNAME"`
	Password string `env:"VALKEY_PASSWORD,REDIS_PASSWORD"`
	DB       int    `env:"VALKEY_DB,REDIS_DB" default:"0"`
	// TLS enables TLS 1.2+ with the host of Addr as server name.
	TLS bool `env:"VALKEY_TLS,REDIS_TLS"`
	// PoolSize caps connections; 0 keeps go-redis' default (10 per CPU).
	PoolSize int `env:"VALKEY_POOL_SIZE"`
	// DialTimeout bounds connecting; 0 keeps go-redis' default (5 s).
	DialTimeout time.Duration `env:"VALKEY_DIAL_TIMEOUT"`
}

// ValkeyConfigFromEnv reads ValkeyConfig from the environment (and .env when
// loaded), with REDIS_* as fallbacks.
func ValkeyConfigFromEnv() (ValkeyConfig, error) {
	return ConfigOf[ValkeyConfig]()
}

// NewValkeyClient builds a client for cfg. It does not dial: the first command
// connects, as with go-redis.
func NewValkeyClient(cfg ValkeyConfig) *redis.Client {
	opts := &redis.Options{
		Addr:        cfg.Addr,
		Username:    cfg.Username,
		Password:    cfg.Password,
		DB:          cfg.DB,
		PoolSize:    cfg.PoolSize,
		DialTimeout: cfg.DialTimeout,
		ClientName:  "nitro",
	}
	if cfg.TLS {
		host, _, err := net.SplitHostPort(cfg.Addr)
		if err != nil {
			host = cfg.Addr
		}
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
	}
	return redis.NewClient(opts)
}

// ValkeyClientFromEnv returns a client when VALKEY_ADDR (or REDIS_ADDR) is set,
// and nil, nil when neither is: Valkey is then simply not configured.
func ValkeyClientFromEnv() (*redis.Client, error) {
	cfg, err := ValkeyConfigFromEnv()
	if err != nil {
		return nil, err
	}
	if cfg.Addr == "" {
		return nil, nil
	}
	return NewValkeyClient(cfg), nil
}

// ValkeyCache is RedisCache over a Valkey client: the shared L2 cache.
func ValkeyCache(client *redis.Client, opts ...RedisCacheOption) Cache {
	return RedisCache(client, opts...)
}

// ValkeySessionStore is RedisSessionStore over a Valkey client.
func ValkeySessionStore(client *redis.Client, ttl time.Duration) SessionStore {
	return RedisSessionStore(client, ttl)
}

// NewValkeyBreakerStore persists named circuit breakers in Valkey.
func NewValkeyBreakerStore(client *redis.Client, keyPrefix string, ttl time.Duration) BreakerStore {
	return NewRedisBreakerStore(client, keyPrefix, ttl)
}
