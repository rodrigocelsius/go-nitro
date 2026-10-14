package domain

import "time"

// RateLimiter controls the request rate.
type RateLimiter interface {
	Allow(key string) bool
	Remaining(key string) int
	Reset(key string)
}

// RateLimitStrategy is the rate limiting strategy.
type RateLimitStrategy string

const (
	// StrategyTokenBucket uses token bucket (current).
	StrategyTokenBucket RateLimitStrategy = "token_bucket"
	// StrategySlidingWindow uses sliding window log.
	StrategySlidingWindow RateLimitStrategy = "sliding_window"
)

// RateLimitConfig configures the limiter.
type RateLimitConfig struct {
	Max      int
	Window   time.Duration
	Key      func(c *Ctx) string
	Strategy RateLimitStrategy
}
