package gateway

import (
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// DefaultCacheTTL is the TTL applied to cached SELECTs when WithCacheTTL is not used.
const DefaultCacheTTL = 5 * time.Minute

// Gateway orchestrates query execution with observability.
type Gateway struct {
	executor SQLExecutor
	tracer   domain.Tracer
	meter    domain.Meter
	logger   domain.Logger
	cache    domain.Cache
	cacheTTL time.Duration
	slow     time.Duration

	// root is set on gateways pinned to a transaction (Pin): the database
	// whose ambient transactions this gateway joins.
	root *Gateway
}

// Config configures the Gateway.
type Config struct {
	Executor SQLExecutor
	Tracer   domain.Tracer
	Meter    domain.Meter
	Logger   domain.Logger
	Cache    domain.Cache

	// Opt-out flags (applied after defaults / With*).
	DisableLogger        bool
	DisableTracer        bool
	DisableMeter         bool
	DisableCache         bool
	DisableObservability bool // logger + tracer + meter + cache

	// CacheTTL is the SELECT cache TTL (default DefaultCacheTTL).
	CacheTTL time.Duration

	// SlowQuery: successful queries log at DEBUG; at or above this duration
	// they log at WARN ("db slow query"). Default NITRO_DB_SLOW_MS or 200ms.
	SlowQuery time.Duration
}

// Option is a functional configuration option.
type Option func(*Config)

// WithTracer replaces the default tracer (e.g. OpenTelemetry).
func WithTracer(t domain.Tracer) Option {
	return func(c *Config) { c.Tracer = t }
}

// WithMeter replaces the default meter (e.g. OpenTelemetry).
func WithMeter(m domain.Meter) Option {
	return func(c *Config) { c.Meter = m }
}

// WithLogger replaces the default logger (slog stderr).
func WithLogger(l domain.Logger) Option {
	return func(c *Config) { c.Logger = l }
}

// WithCache enables SELECT caching (off by default).
func WithCache(c domain.Cache) Option {
	return func(cfg *Config) { cfg.Cache = c }
}

// WithCacheTTL sets the SELECT cache TTL (default DefaultCacheTTL).
func WithCacheTTL(d time.Duration) Option {
	return func(cfg *Config) { cfg.CacheTTL = d }
}

// WithSlowQuery sets the duration from which a successful query logs at WARN.
func WithSlowQuery(d time.Duration) Option {
	return func(c *Config) { c.SlowQuery = d }
}

// DisableLogger turns off query logs.
func DisableLogger() Option {
	return func(c *Config) { c.DisableLogger = true }
}

// DisableTracer turns off DB spans.
func DisableTracer() Option {
	return func(c *Config) { c.DisableTracer = true }
}

// DisableMeter turns off DB metrics.
func DisableMeter() Option {
	return func(c *Config) { c.DisableMeter = true }
}

// DisableCache turns off cache (if WithCache was used).
func DisableCache() Option {
	return func(c *Config) { c.DisableCache = true }
}

// DisableObservability turns off logger, tracer, meter, and cache.
func DisableObservability() Option {
	return func(c *Config) { c.DisableObservability = true }
}

// New creates a Gateway with observability on by default:
//   - Logger: stderr, NITRO_LOG_FORMAT/NITRO_LOG_LEVEL; queries at DEBUG, slow ones (NITRO_DB_SLOW_MS) at WARN
//   - Tracer / Meter: domain.DefaultTracer/Meter (OTel after otel.Ensure; otherwise nop)
//   - Cache: off (use WithCache)
//
// Use DisableLogger / DisableTracer / DisableMeter / DisableObservability to opt out.
// Env: NITRO_DB_QUIET=1 (pgx.Open) turns everything off.
func New(executor SQLExecutor, opts ...Option) *Gateway {
	cfg := Config{
		Executor:  executor,
		Logger:    defaultLogger(),
		Tracer:    defaultTracer(),
		Meter:     defaultMeter(),
		SlowQuery: domain.SlowQueryFromEnv(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	applyDisables(&cfg)
	ttl := cfg.CacheTTL
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	return &Gateway{
		executor: cfg.Executor,
		tracer:   cfg.Tracer,
		meter:    cfg.Meter,
		logger:   cfg.Logger,
		cache:    cfg.Cache,
		cacheTTL: ttl,
		slow:     cfg.SlowQuery,
	}
}

// selectTTL returns the configured SELECT cache TTL.
func (g *Gateway) selectTTL() time.Duration {
	if g.cacheTTL <= 0 {
		return DefaultCacheTTL
	}
	return g.cacheTTL
}

func applyDisables(cfg *Config) {
	if cfg.DisableObservability {
		cfg.Logger = nil
		cfg.Tracer = nil
		cfg.Meter = nil
		cfg.Cache = nil
		return
	}
	if cfg.DisableLogger {
		cfg.Logger = nil
	}
	if cfg.DisableTracer {
		cfg.Tracer = nil
	}
	if cfg.DisableMeter {
		cfg.Meter = nil
	}
	if cfg.DisableCache {
		cfg.Cache = nil
	}
}

// Executor returns the underlying SQLExecutor (escape hatch; prefer Query/Exec/QueryRow).
func (g *Gateway) Executor() SQLExecutor {
	return g.executor
}

// LoggerEnabled reports whether DB logs are active.
func (g *Gateway) LoggerEnabled() bool { return g.logger != nil }

// TracerEnabled reports whether DB tracing is active.
func (g *Gateway) TracerEnabled() bool { return g.tracer != nil }

// MeterEnabled reports whether DB metrics are active.
func (g *Gateway) MeterEnabled() bool { return g.meter != nil }

// CacheEnabled reports whether SELECT cache is active.
func (g *Gateway) CacheEnabled() bool { return g.cache != nil }
