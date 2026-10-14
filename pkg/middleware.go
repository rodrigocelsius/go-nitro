package nitro

import (
	"time"

	nitroadapter "github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// === Middleware Constructors ===

// CORSMiddleware returns middleware that configures CORS.
// With no config, it allows all origins in development
// and requires explicit configuration in production.
func CORSMiddleware(cfg ...CORSConfig) Handler {
	var dc []domain.CORSConfig
	for _, c := range cfg {
		dc = append(dc, domain.CORSConfig(c))
	}
	mw := nitroadapter.CORSMiddleware(dc...)
	return wrapAdapter(mw)
}

// CompressMiddleware returns gzip compression middleware.
// Must be registered as post-middleware via UsePost.
func CompressMiddleware(cfg ...CompressConfig) Handler {
	mw := nitroadapter.CompressMiddleware(cfg...)
	return wrapAdapter(mw)
}

// RecoverMiddleware returns middleware that recovers from panics.
func RecoverMiddleware() Handler {
	mw := nitroadapter.RecoverMiddleware()
	return wrapAdapter(mw)
}

// RecoverMiddlewareWithDebug returns panic-recovery middleware whose
// stack-trace exposure in 500 responses is controlled by an explicit
// debug flag (see Config.Debug), decoupled from Env.
func RecoverMiddlewareWithDebug(debug bool) Handler {
	mw := nitroadapter.RecoverMiddlewareWithDebug(debug)
	return wrapAdapter(mw)
}

// TimeoutMiddleware returns per-request timeout middleware.
func TimeoutMiddleware(timeout time.Duration) Handler {
	mw := nitroadapter.TimeoutMiddleware(timeout)
	return wrapAdapter(mw)
}

// RateLimitKeyFunc builds the rate-limit bucket key from the request context.
type RateLimitKeyFunc func(c *Ctx) string

// RateLimit returns sliding-window rate limiting middleware keyed by client IP.
// max: maximum requests per window.
// window: time window (e.g. time.Minute).
func RateLimit(max int, window time.Duration) Handler {
	return RateLimitWith(max, window, nil)
}

// RateLimitWith is like RateLimit but uses a custom key function.
// If keyFunc is nil, the client IP is used (same as RateLimit).
//
// Helpers: IPKeyFunc, APIKeyKeyFunc, CompositeKeyFunc, HeaderKeyFunc.
//
//	app.Use(nitro.RateLimitWith(1000, time.Minute, nitro.CompositeKeyFunc()))
func RateLimitWith(max int, window time.Duration, keyFunc RateLimitKeyFunc) Handler {
	inner := nitroadapter.DefaultKeyFunc
	if keyFunc != nil {
		inner = func(dc *domain.Ctx) string {
			if fc, ok := dc.AdapterData.(*nitroadapter.FastCtx); ok && fc != nil {
				return keyFunc(fc)
			}
			return nitroadapter.DefaultKeyFunc(dc)
		}
	}
	mw := nitroadapter.SlidingWindowRateLimitMiddleware(max, window, inner)
	return wrapAdapter(mw)
}

// IPKeyFunc rate-limits by remote IP.
func IPKeyFunc() RateLimitKeyFunc {
	inner := nitroadapter.IPKeyFunc()
	return func(c *Ctx) string { return inner(&c.Ctx) }
}

// APIKeyKeyFunc rate-limits by X-API-Key header.
func APIKeyKeyFunc() RateLimitKeyFunc {
	inner := nitroadapter.APIKeyKeyFunc()
	return func(c *Ctx) string { return inner(&c.Ctx) }
}

// CompositeKeyFunc rate-limits by IP + X-API-Key when present.
func CompositeKeyFunc() RateLimitKeyFunc {
	inner := nitroadapter.CompositeKeyFunc()
	return func(c *Ctx) string { return inner(&c.Ctx) }
}

// HeaderKeyFunc rate-limits by an arbitrary request header.
func HeaderKeyFunc(header string) RateLimitKeyFunc {
	inner := nitroadapter.HeaderKeyFunc(header)
	return func(c *Ctx) string { return inner(&c.Ctx) }
}

// === Phase 4: Logger ===

// LoggerConfig configures the access-log middleware.
type LoggerConfig = nitroadapter.LoggerConfig

// Logger registers access-log middleware (pre + post) directly on the adapter
// (without a double wrapHandler).
//
//	app.Logger() // Fiber-style text (colors on TTY)
//	app.Logger(nitro.LoggerConfig{Format: "json", Async: true})
//	app.Logger(nitro.LoggerConfig{Format: "plain"}) // method path status ms
//	app.Logger(nitro.LoggerConfig{Colors: "always"})
func (a *App) Logger(cfg ...LoggerConfig) *App {
	pre, post := nitroadapter.Logger(cfg...)
	a.inner.Use(pre)
	a.inner.UsePost(post)
	return a
}

// UseDomain registers a domain.Handler without the facade wrapHandler layer.
// Use for middlewares already implemented in the adapter.
func (a *App) UseDomain(mw domain.Handler) *App {
	a.inner.Use(mw)
	return a
}

// UsePostDomain registers a post-middleware domain.Handler without wrapHandler.
func (a *App) UsePostDomain(mw domain.Handler) *App {
	a.inner.UsePost(mw)
	return a
}

// LoggerPre returns a pre-middleware that stores the request start time.
// Combine with LoggerPost for full logging:
//
//	app.Use(nitro.LoggerPre())
//	app.UsePost(nitro.LoggerPost())
func LoggerPre(cfg ...LoggerConfig) Handler {
	pre := nitroadapter.LoggerPre(cfg...)
	return wrapAdapter(pre)
}

// LoggerPost returns a post-middleware that logs the request.
func LoggerPost(cfg ...LoggerConfig) Handler {
	post := nitroadapter.LoggerPost(cfg...)
	return wrapAdapter(post)
}

// === Phase 4: IPWhiteList ===

// IPWhiteList returns middleware that rejects IPs not in the list.
// Supports single IPs and CIDR ranges (e.g. "192.168.1.0/24").
//
//	app.Use(nitro.IPWhiteList("127.0.0.1", "10.0.0.0/8"))
func IPWhiteList(ips ...string) Handler {
	mw := nitroadapter.IPWhitelistMiddleware(ips...)
	return wrapAdapter(mw)
}

// TrustedHost returns middleware that rejects requests whose Host header
// isn't one of allowedHosts — apply before anything that builds links from
// c.Hostname()/c.BaseURL() (password reset, email verification, redirects)
// to prevent Host header injection/poisoning. Supports exact hosts and
// "*.example.com" wildcard subdomains (also matches the apex domain).
//
//	app.Use(nitro.TrustedHost("app.example.com", "*.example.com"))
func TrustedHost(allowedHosts ...string) Handler {
	mw := nitroadapter.TrustedHostMiddleware(allowedHosts...)
	return wrapAdapter(mw)
}

// TenantConfig configures TenantMiddleware (shared-schema multi-tenancy).
type TenantConfig = nitroadapter.TenantConfig

// TenantMiddleware resolves the current tenant into request locals so
// c.TenantID() and ResourceConfig.TenantScope work for the rest of the chain.
// Prefer after JWTMiddleware when tenants come from JWT claims.
//
//	app.Use(nitro.JWTMiddleware(provider))
//	app.Use(nitro.TenantMiddleware(nitro.TenantConfig{Require: true}))
func TenantMiddleware(cfg ...TenantConfig) Handler {
	mw := nitroadapter.TenantMiddleware(cfg...)
	return wrapAdapter(mw)
}

// DetectFileType sniffs the actual content type of uploaded/stored bytes
// from their first bytes (magic-byte based), independent of any
// client-supplied filename extension or Content-Type header — both are
// trivially spoofable. Call on the bytes returned by
// (*Ctx).FormFile(key).ReadAll() before SaveFile/further processing.
//
//	fh, _ := c.FormFile("avatar")
//	data, _ := fh.ReadAll()
//	if !strings.HasPrefix(nitro.DetectFileType(data), "image/") {
//		return c.BadRequest("only images are allowed")
//	}
func DetectFileType(data []byte) string {
	return nitroadapter.DetectFileType(data)
}

// === Phase 4: Dashboard ===

// Dashboard registers the admin dashboard for metrics and routes.
// prefix: base prefix (e.g. "/admin/nitro").
//
//	app.Dashboard("/admin/nitro", nitro.IPWhiteList("10.0.0.0/8"))
//	// UI:  GET /admin/nitro/dashboard
//	// API: GET /admin/nitro/api/dashboard
//
// In production (Env == EnvProduction) at least one protecting middleware is
// REQUIRED (IP allow-list, JWT + role check, ...): Dashboard panics otherwise.
func (a *App) Dashboard(prefix string, mws ...Handler) *App {
	a.inner.EnableMetrics()
	cfg := nitroadapter.DashboardConfig{
		Enabled:    true,
		BasePath:   prefix,
		Middleware: wrapHandlers(mws),
	}
	nitroadapter.RegisterDashboard(a.inner, cfg)
	return a
}

// === Phase 4: Metrics / Prometheus ===

// MetricsCollector collects request and middleware metrics.
type MetricsCollector = nitroadapter.MetricsCollector

// Metrics enables metrics collection on the app.
// Prefer app.Prometheus() if you also want the /metrics endpoint.
//
//	app.Metrics()
//	app.Get("/metrics", nitro.PrometheusHandler(app.MetricsCollector()))
func (a *App) Metrics() *App {
	a.inner.EnableMetrics()
	return a
}

// MetricsCollector returns the metrics collector (nil if not enabled).
func (a *App) MetricsCollector() *MetricsCollector {
	return a.inner.Metrics()
}

// EnableMetrics enables metrics collection and returns the collector.
func (a *App) EnableMetrics() *MetricsCollector {
	return a.inner.EnableMetrics()
}

// Prometheus registers the Prometheus-format metrics endpoint.
// Enables collection automatically. Default path: "/metrics".
//
//	app.Prometheus()
//	app.Prometheus("/metrics")
func (a *App) Prometheus(path ...string) *App {
	p := "/metrics"
	if len(path) > 0 && path[0] != "" {
		p = path[0]
	}
	a.inner.EnableMetrics()
	a.Get(p, a.MetricsHandler())
	return a
}

// MetricsHandler returns a handler that serves Prometheus text metrics.
func (a *App) MetricsHandler() Handler {
	mc := a.inner.Metrics()
	if mc == nil {
		mc = a.inner.EnableMetrics()
	}
	return PrometheusHandler(mc)
}

// PrometheusHandler returns a handler that serves Prometheus metrics.
//
//	app.Get("/metrics", nitro.PrometheusHandler(app.MetricsCollector()))
func PrometheusHandler(mc *MetricsCollector) Handler {
	mw := nitroadapter.PrometheusHandler(mc)
	return wrapAdapter(mw)
}

// CORS enables CORS for the whole app (chainable). With no config it follows
// the CORSMiddleware convention: permissive in development, explicit
// AllowedOrigins required in production.
//
//	app.CORS(nitro.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}})
func (a *App) CORS(cfg ...CORSConfig) *App {
	return a.Use(CORSMiddleware(cfg...))
}

// Compress enables gzip compression for the whole app (chainable). It is
// registered as a post-middleware automatically — the placement users most
// often get wrong with the raw CompressMiddleware.
//
//	app.Compress()
func (a *App) Compress(cfg ...CompressConfig) *App {
	return a.UsePost(CompressMiddleware(cfg...))
}

// Timeout aborts requests slower than d with 408 for the whole app (chainable).
//
//	app.Timeout(10 * time.Second)
func (a *App) Timeout(d time.Duration) *App {
	return a.Use(TimeoutMiddleware(d))
}

// RateLimit limits each client IP to max requests per window for the whole
// app (chainable). The client IP is c.ClientIP(): correct behind a reverse
// proxy with no extra configuration.
//
//	app.RateLimit(100, time.Minute)
func (a *App) RateLimit(max int, window time.Duration) *App {
	return a.Use(RateLimit(max, window))
}
