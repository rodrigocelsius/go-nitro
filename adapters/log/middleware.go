package log

import (
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// LoggingMiddleware returns middleware that injects a logger
// with request_id into the context.
func LoggingMiddleware(logger domain.Logger) domain.Handler {
	return func(c *domain.Ctx) error {
		reqID := c.RequestID()
		if reqID != "" {
			c.SetLogger(logger.With("request_id", reqID))
		} else {
			c.SetLogger(logger)
		}
		return nil
	}
}

// RequestLoggerConfig configures the RequestLogger middleware.
type RequestLoggerConfig struct {
	// Logger is the logger used to record requests.
	// If nil, uses c.Log() from the context.
	Logger domain.Logger
	// ExcludePaths lists paths that must not be logged.
	ExcludePaths []string
}

type startTimeSetter interface {
	SetStartTime(t time.Time)
}

type startTimeGetter interface {
	StartTime() time.Time
}

// RequestLogger returns middleware that stores the start instant
// so the RequestLoggerPost post-middleware can log duration.
func RequestLogger(cfg ...RequestLoggerConfig) domain.Handler {
	config := buildConfig(cfg)
	return func(c *domain.Ctx) error {
		if config.excludes[string(c.Path())] {
			return nil
		}
		if s, ok := c.AdapterData.(startTimeSetter); ok {
			s.SetStartTime(time.Now())
		}
		return nil
	}
}

// RequestLoggerPost is the post-middleware that logs the request with
// method, path, status code, and duration.
func RequestLoggerPost(cfg ...RequestLoggerConfig) domain.Handler {
	config := buildConfig(cfg)
	return func(c *domain.Ctx) error {
		if config.excludes[string(c.Path())] {
			return nil
		}

		logger := config.logger
		if logger == nil {
			logger = c.Log()
		}
		if logger == nil {
			return nil
		}

		start := getStartTime(c)
		duration := time.Since(start)

		logger.Info("request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Status(),
			"duration", duration.String(),
		)
		return nil
	}
}

func buildConfig(cfg []RequestLoggerConfig) struct {
	logger   domain.Logger
	excludes map[string]bool
} {
	type config struct {
		logger   domain.Logger
		excludes map[string]bool
	}
	c := config{}
	if len(cfg) > 0 {
		c.logger = cfg[0].Logger
		if len(cfg[0].ExcludePaths) > 0 {
			c.excludes = make(map[string]bool, len(cfg[0].ExcludePaths))
			for _, p := range cfg[0].ExcludePaths {
				c.excludes[p] = true
			}
		}
	}
	return c
}

func getStartTime(c *domain.Ctx) time.Time {
	if s, ok := c.AdapterData.(startTimeGetter); ok {
		if t := s.StartTime(); !t.IsZero() {
			return t
		}
	}
	return time.Now()
}
