package nitro

import (
	"context"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

const defaultPingTimeout = 2 * time.Second

// Ping wraps a context-aware ping in NamedHealth with a short timeout (default 2s).
//
//	app.HealthCheck(nitro.Ping("postgres", db.Pool.Ping))
//	app.HealthCheck(nitro.Ping("redis", func(ctx context.Context) error { return rdb.Ping(ctx).Err() }))
func Ping(name string, ping func(context.Context) error) domain.HealthChecker {
	return PingTimeout(name, defaultPingTimeout, ping)
}

// PingTimeout is Ping with a custom timeout.
func PingTimeout(name string, timeout time.Duration, ping func(context.Context) error) domain.HealthChecker {
	if timeout <= 0 {
		timeout = defaultPingTimeout
	}
	return NamedHealth(name, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if ping == nil {
			return nil
		}
		return ping(ctx)
	})
}
