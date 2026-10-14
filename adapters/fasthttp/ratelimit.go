package fasthttp

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// RateLimitMiddleware returns middleware that rate-limits requests
// and includes X-RateLimit-* headers on the response.
func RateLimitMiddleware(limiter domain.RateLimiter, max int, keyFunc func(*domain.Ctx) string) domain.Handler {
	return func(c *domain.Ctx) error {
		key := keyFunc(c)

		if !limiter.Allow(key) {
			fc, _ := c.AdapterData.(*FastCtx)
			if fc != nil {
				fc.SetHeader("X-RateLimit-Limit", strconv.Itoa(max))
				fc.SetHeader("X-RateLimit-Remaining", "0")
			}
			return &domain.TooManyRequestsError{}
		}

		remaining := limiter.Remaining(key)
		c.SetHeader("X-RateLimit-Limit", strconv.Itoa(max))
		c.SetHeader("X-RateLimit-Remaining", strconv.Itoa(remaining))
		return nil
	}
}

// DefaultKeyFunc returns the default rate-limit key: the real client IP
// (FastCtx.ClientIP). Behind a reverse proxy in a private network each client
// gets its own bucket without configuration; forwarded headers are only read
// from trusted proxies, so they cannot be forged to dodge the limit.
func DefaultKeyFunc(c *domain.Ctx) string {
	if fc, ok := c.AdapterData.(*FastCtx); ok && fc != nil {
		return fc.ClientIP()
	}
	return "unknown"
}

// TrustedProxyKeyFunc returns a function that uses the real client IP
// as the key, considering X-Forwarded-For only when the direct IP
// is a trusted proxy. Prevents rate-limit bypass via header spoofing.
func TrustedProxyKeyFunc(trustedProxies ...string) func(*domain.Ctx) string {
	trustedNets := make([]*net.IPNet, 0, len(trustedProxies))
	trustedIPs := make(map[string]bool)
	for _, entry := range trustedProxies {
		if _, ipnet, err := net.ParseCIDR(entry); err == nil {
			trustedNets = append(trustedNets, ipnet)
		} else if ip := net.ParseIP(entry); ip != nil {
			trustedIPs[ip.String()] = true
		}
	}

	return func(c *domain.Ctx) string {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return "unknown"
		}
		fctx := fc.FasthttpCtx()
		directIP := net.ParseIP(fctx.RemoteIP().String())
		if directIP == nil {
			return "unknown"
		}

		// Only use X-Forwarded-For if direct IP is trusted
		if isTrustedProxy(directIP, trustedNets, trustedIPs) {
			xff := string(fctx.Request.Header.Peek("X-Forwarded-For"))
			if xff != "" {
				// Rightmost-untrusted: proxies APPEND the caller's address,
				// so left-side entries are attacker-controlled. Walking from
				// the right and skipping trusted hops yields an IP that
				// cannot be forged (the old leftmost pick allowed bypass by
				// rotating a fake header).
				if ip := rightmostUntrustedIP(xff, trustedNets, trustedIPs); ip != "" {
					return ip
				}
			}
		}

		return directIP.String()
	}
}

// rightmostUntrustedIP walks X-Forwarded-For from right to left and returns
// the first entry outside the trusted set. Returns "" when every entry is a
// trusted proxy (caller falls back to the direct IP).
func rightmostUntrustedIP(xff string, nets []*net.IPNet, ips map[string]bool) string {
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(parts[i])
		ip := net.ParseIP(candidate)
		if ip == nil {
			continue // malformed entry: skip
		}
		if !isTrustedProxy(ip, nets, ips) {
			return candidate
		}
	}
	return ""
}

// IPKeyFunc returns a function that uses IP as the key.
func IPKeyFunc() func(*domain.Ctx) string {
	return DefaultKeyFunc
}

// HeaderKeyFunc returns a function that uses a header as the key.
func HeaderKeyFunc(header string) func(*domain.Ctx) string {
	return func(c *domain.Ctx) string {
		return c.HeaderParam(header)
	}
}

// APIKeyKeyFunc returns a function that uses the X-API-Key header as the key.
func APIKeyKeyFunc() func(*domain.Ctx) string {
	return HeaderKeyFunc("X-API-Key")
}

// CompositeKeyFunc returns a function that combines IP + API Key.
// If the X-API-Key header is present, uses "ip:apikey".
// Otherwise uses IP only.
func CompositeKeyFunc() func(*domain.Ctx) string {
	return func(c *domain.Ctx) string {
		ip := DefaultKeyFunc(c)
		apiKey := c.HeaderParam("X-API-Key")
		if apiKey != "" {
			return ip + ":" + apiKey
		}
		return ip
	}
}

// SlidingWindowRateLimitMiddleware returns sliding-window rate-limit middleware.
func SlidingWindowRateLimitMiddleware(max int, window time.Duration, keyFunc func(*domain.Ctx) string) domain.Handler {
	limiter := NewSlidingWindowLimiter(max, window)
	return RateLimitMiddleware(limiter, max, keyFunc)
}

// NewSlidingWindowLimiter creates a sliding-window limiter for middleware use.
// Delegates to the ratelimit adapter.
func NewSlidingWindowLimiter(max int, window time.Duration) domain.RateLimiter {
	return newSlidingWindowAdapter(max, window)
}
