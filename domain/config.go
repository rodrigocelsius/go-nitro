package domain

import (
	"crypto/tls"
	"time"
)

// Env represents the application runtime environment. Used by the
// security layer (adapters/security) to decide environment-sensitive
// behavior — for example, HSTS is disabled in EnvDevelopment because
// it forces HTTPS in the browser, which would break a local server
// running plain HTTP (a common development default).
type Env int

const (
	// EnvProduction is the default when Config.Env is unspecified —
	// follows "secure by default": without an explicit setting,
	// the most restrictive environment is assumed.
	EnvProduction Env = iota
	EnvDevelopment
)

// SecurityConfig controls the headers and protections enabled by default
// by the security layer. The zero value (SecurityConfig{}) is already the
// full secure default — the only way to turn something off is to set the
// specific field to true/customized.
type SecurityConfig struct {
	// DisableHeaders turns off all security headers
	// (X-Content-Type-Options, X-Frame-Options, Referrer-Policy, HSTS,
	// CSP). Use with caution — usually only makes sense if another
	// layer (reverse proxy, CDN) already handles this.
	DisableHeaders bool

	// DisableHSTS turns off only the Strict-Transport-Security header,
	// keeping the rest. Automatically disabled when
	// Config.Env == EnvDevelopment, even without setting this here.
	DisableHSTS bool

	// CSP customizes the Content-Security-Policy.
	// If empty, the restrictive default is used: "default-src 'self'".
	CSP string

	// CORS configures Cross-Origin Resource Sharing. By default
	// (CORSConfig{}), NO cross-origin origin is allowed.
	CORS CORSConfig

	// AllowWildcardCORSInProd must be explicitly set to true to boot with
	// Env == EnvProduction while CORS AllowedOrigins is "*" (e.g. via
	// nitro.PublicStack). Without it, the app fails fast at boot instead
	// of silently accepting cross-origin requests from any site in
	// production — see vuln.md (V-14).
	AllowWildcardCORSInProd bool

	// AllowUnauthenticatedResources must be explicitly set to true to boot
	// in EnvProduction with app.Resource routes that have neither auth
	// Middleware nor ResourceConfig.Public=true. Without it, registration
	// panics (V-04) instead of only logging a warning. Prefer
	// app.SetDefaultResourceMiddleware(JWT…) or Public:true per Resource.
	AllowUnauthenticatedResources bool
}

// CORSConfig controls the Cross-Origin Resource Sharing policy.
type CORSConfig struct {
	// AllowedOrigins lists allowed origins (e.g. "https://app.example.com").
	// Empty (default) = no cross-origin origin allowed.
	AllowedOrigins []string

	// AllowedMethods lists HTTP methods allowed in cross-origin
	// requests. Default when empty: GET, POST, PUT, DELETE, OPTIONS.
	AllowedMethods []string

	// AllowedHeaders lists headers the client may send.
	// Default when empty: Content-Type, Authorization.
	AllowedHeaders []string

	// AllowCredentials enables the Access-Control-Allow-Credentials header.
	// Cannot be used together with AllowedOrigins=["*"].
	AllowCredentials bool

	// MaxAge is how long (in seconds) the browser may cache the
	// preflight (OPTIONS) response. Default: 600 (10 min).
	MaxAge int
}

// Config holds App initialization options.
type Config struct {
	// UseLinearRouter forces the linear-search router instead of the
	// optimized one (hash + radix).
	// Exists as a safety net during the phase transition and for
	// comparative benchmarks — must not be used in production.
	UseLinearRouter bool

	// Env is the runtime environment. Default: EnvProduction (most
	// restrictive) when unspecified — see Env.
	Env Env

	// Debug explicitly enables verbose diagnostics — currently, including
	// sanitized panic stack traces in 500 response bodies (see
	// RecoverMiddlewareWithDebug). Deliberately kept separate from Env:
	// Env also gates CORS/HSTS/other hardening, and a staging or
	// internally-reachable deployment may legitimately need
	// Env == EnvDevelopment for those without also wanting to leak stack
	// traces to any client that can reach it (vuln.md V-33). Defaults to
	// false regardless of Env.
	Debug bool

	// Security customizes headers/protections. Zero value
	// (SecurityConfig{}) is already the full secure default — only
	// needed to turn something off explicitly.
	Security SecurityConfig

	// RequestTimeout is the maximum execution time per request.
	// Requests that exceed this limit receive status 408.
	// Zero (default) means no timeout.
	RequestTimeout time.Duration

	// TLSConfig configures the TLS server.
	// When set, Listen/ListenTLS apply this configuration.
	TLSConfig *tls.Config

	// ErrorHandler overrides the global error handler.
	// If set, it is called for every error returned by handlers
	// and middlewares.
	// The handler may customize the response (log, format, status).
	// If it returns nil, the response was already written manually.
	ErrorHandler func(c *Ctx, err error) error

	// TrustedProxies lists the reverse proxies / load balancers whose
	// X-Forwarded-For, X-Real-IP and X-Forwarded-Proto headers are honored by
	// c.ClientIP() and c.Protocol(). Entries are IPs, CIDRs or "private".
	//
	// Convention (nil, the default): loopback and private networks
	// (10/8, 172.16/12, 192.168/16, fc00::/7, link-local) are trusted — what a
	// reverse proxy, Docker or Kubernetes ingress looks like — while public
	// peers can never forge the headers. Set it only for a public edge
	// (e.g. Cloudflare ranges); an empty non-nil slice trusts no proxy at all.
	TrustedProxies []string

	// DisableRequestID turns off the RequestID middleware.
	// Useful for benchmarks where UUID overhead is unwanted.
	DisableRequestID bool

	// DisableDefaults turns off Recover, Logger, MaxBody and development CORS
	// applied automatically by nitro.New(). RequestID and security headers
	// remain active (use DisableRequestID / Security.DisableHeaders).
	DisableDefaults bool

	// EnableLogger records access log even with DisableDefaults, or reinforces
	// the default logger (idempotent if Defaults already applied Logger).
	EnableLogger bool

	// EnablePrometheus registers the Prometheus endpoint (default /metrics).
	EnablePrometheus bool

	// PrometheusPath overrides the endpoint path (only with EnablePrometheus).
	PrometheusPath string

	// DisableTelemetry turns off OpenTelemetry (HTTP + global Tracer/Meter) via
	// otel.ForceDisable — also prevents pgx.Open from reinstalling OTel.
	// Opt-out via env: NITRO_OTEL=0 or OTEL_SDK_DISABLED=true.
	// DB only: NITRO_DB_QUIET=1 in pgx.Open.
	DisableTelemetry bool

	// ServiceName used in the OpenTelemetry resource (default: OTEL_SERVICE_NAME or "nitro").
	ServiceName string

	// DisableGracefulShutdown makes Listen() block without handling SIGINT/SIGTERM.
	// By default Listen() performs graceful shutdown (30s timeout).
	DisableGracefulShutdown bool

	// DisableStartupMessage omits the ASCII banner (Fiber-style) on Listen.
	DisableStartupMessage bool

	// MaxRequestBodySize caps how many bytes the underlying fasthttp.Server
	// will buffer per request, at the socket/connection level — *before*
	// any handler or MaxBodySizeMiddleware runs. Without this, fasthttp's
	// own default (4 MiB) applies, so a body between the app's intended
	// limit (e.g. MaxBodySizeMiddleware(1<<20)) and 4 MiB is still fully
	// read into memory per connection before being rejected, weakening
	// the app-level limit as a memory-exhaustion defense. Zero (default)
	// uses DefaultMaxRequestBodySize (4 MiB, fasthttp's own default) —
	// set this to match (or tighten) your MaxBodySizeMiddleware limit.
	MaxRequestBodySize int

	// DisableInlineRecover turns off recover in the fasthttp Handler (defer).
	// By default the Handler always recovers panics. With Defaults(),
	// RecoverMiddleware already covers this — use true in benchmarks (like Fiber).
	DisableInlineRecover bool

	// Validator is this App's validation engine (validate tags + custom rules).
	// If nil, the adapter creates an isolated NewValidator() (builtins, StrictUnknown=true).
	// Use nitro.NewValidator() + Register("cpf", …) and pass it here, or app.SetValidator.
	Validator Validator

	// Cache is the App's default L2 cache (e.g. Redis). When set, Resource
	// inherits Cache + CacheBustPattern (derived from the path) and turns off in-memory L1,
	// unless ResourceConfig sets CacheTTL > 0 or Cache/CacheBustPattern
	// explicitly. Equivalent to app.SetCache after New.
	Cache Cache
}
