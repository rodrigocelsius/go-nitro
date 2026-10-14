// Package nitro is the public surface of the framework, providing a
// unified API that re-exports types and functions from internal layers.
//
// This package is the Clean Architecture facade, so
// users can import everything from a single place:
//
//	import "github.com/rodrigocelsius/go-nitro/pkg/v2"
//
//	app := nitro.New()
//	app.Get("/users/:id", func(c *nitro.Ctx) error {
//	    return c.SendString("Hello " + c.Param("id"))
//	})
//	app.Listen(":8080")
package nitro

import (
	"context"
	"log"
	"os"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"github.com/redis/go-redis/v9"
	bcryptadapter "github.com/rodrigocelsius/go-nitro/adapters/auth/bcrypt/v2"
	jwtadapter "github.com/rodrigocelsius/go-nitro/adapters/auth/jwt/v2"
	keycloakadapter "github.com/rodrigocelsius/go-nitro/adapters/auth/keycloak/v2"
	oauth2adapter "github.com/rodrigocelsius/go-nitro/adapters/auth/oauth2/v2"
	oidcadapter "github.com/rodrigocelsius/go-nitro/adapters/auth/oidc/v2"
	sessionadapter "github.com/rodrigocelsius/go-nitro/adapters/auth/session/v2"
	cacheadapter "github.com/rodrigocelsius/go-nitro/adapters/cache/v2"
	nitroadapter "github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2"
	oteladapter "github.com/rodrigocelsius/go-nitro/adapters/otel/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/ports/v2"
	nitroscheduler "github.com/rodrigocelsius/go-nitro/scheduler/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

// === Domain Types (re-exports) ===

// Map is a shortcut for map[string]any in JSON responses.
type Map = map[string]any

// Ctx is the Nitro request context, with full
// HTTP support (method, path, body, JSON, route params).
type Ctx = nitroadapter.FastCtx

// Handler is the route handler signature.
// It accepts *Ctx (FastCtx), which exposes all HTTP methods.
type Handler func(c *Ctx) error

// RouteInfo describes a registered route.
type RouteInfo = domain.RouteInfo

// Config holds App initialization options.
type Config = domain.Config

// Env represents the runtime environment.
type Env = domain.Env

// SecurityConfig controls security headers.
type SecurityConfig = domain.SecurityConfig

// CORSConfig controls CORS.
type CORSConfig = domain.CORSConfig

// HTTPError is the interface for HTTP errors.
type HTTPError = domain.HTTPError

// ResourceListResult is the paginated result returned by List.
type ResourceListResult = domain.ResourceListResult

// ListQuery / QueryableLister — Resource List with BLinq push-down (pgx.AdaptStore).
type ListQuery = domain.ListQuery
type ListFilter = domain.ListFilter
type ListSort = domain.ListSort
type QueryableLister = domain.QueryableLister

// CompressConfig configures the compression middleware.
type CompressConfig = nitroadapter.CompressConfig

// GracefulShutdownConfig configures graceful shutdown behavior.
type GracefulShutdownConfig = nitroadapter.GracefulShutdownConfig

// StaticConfig configures the static file server (SPA via SinglePageApp).
type StaticConfig = nitroadapter.StaticConfig

// Hooks groups lifecycle callbacks (OnListen, OnShutdown, OnRoute).
type Hooks = nitroadapter.Hooks

// ListenData is passed to OnListen callbacks.
type ListenData = nitroadapter.ListenData

// === WebSocket Types (re-exports) ===

// WebSocket is the interface for WebSocket connections.
type WebSocket = domain.WebSocket

// WSHandler is the WebSocket handler function.
type WSHandler = domain.WSHandler

// WSHub manages WebSocket connections with broadcasting and rooms.
type WSHub = domain.WSHub

// WebSocketHandler creates a handler for WebSocket upgrade.
func WebSocketHandler(fn func(ws WebSocket)) Handler {
	mw := nitroadapter.WebSocketHandler(func(ws domain.WebSocket) error {
		fn(ws)
		return nil
	})
	return wrapAdapter(mw)
}

// WebSocketHandlerWithConfig creates a WebSocket upgrade handler with config.
func WebSocketHandlerWithConfig(fn func(ws WebSocket), cfg nitroadapter.WSConfig) Handler {
	mw := nitroadapter.WebSocketHandlerWithConfig(func(ws domain.WebSocket) error {
		fn(ws)
		return nil
	}, cfg)
	return wrapAdapter(mw)
}

// NewWSHub creates a hub to manage WebSocket connections.
func NewWSHub() *nitroadapter.FastWSHub {
	return nitroadapter.NewWSHub()
}

// === SSE Types (re-exports) ===

// SSE is the Server-Sent Events interface.
type SSE = domain.SSE

// SSEHandler is the SSE handler function.
type SSEHandler = domain.SSEHandler

// SSECreate creates a Server-Sent Events handler.
func SSECreate(fn func(sse SSE)) Handler {
	mw := nitroadapter.SSEHandler(func(sse domain.SSE) error {
		fn(sse)
		return nil
	})
	return wrapAdapter(mw)
}

// === Constants (re-exports) ===

const (
	// EnvProduction is the production environment (default).
	EnvProduction = domain.EnvProduction
	// EnvDevelopment is the development environment.
	EnvDevelopment = domain.EnvDevelopment
	// MaxParams is the maximum number of route params without allocation.
	MaxParams = domain.MaxParams
)

// === Phase 4: Parse[T] ===

// Parse binds and validates the body according to Content-Type (JSON / form / query).
// Returns the typed value and a ready-to-return HTTP error (400/422).
//
//	req, err := nitro.Parse[LoginRequest](c)
//	if err != nil {
//	    return err
//	}
func Parse[T any](c *Ctx) (T, error) {
	var v T
	if err := c.BodyParser(&v); err != nil {
		return v, err
	}
	return v, nil
}

// === Phase 4: JSON Helpers ===

// wrapHandler converts Handler (*FastCtx) → domain.Handler (*domain.Ctx).
// FastCtx embeds domain.Ctx as the first field; &fc.Ctx and fc share
// the same address — zero-cost conversion (no type assert / alloc).
func wrapHandler(h Handler) domain.Handler {
	return func(c *domain.Ctx) error {
		return h(AsCtx(c))
	}
}

// AsCtx recovers *Ctx (*FastCtx) from *domain.Ctx without allocation.
// Safe when c comes from the HTTP adapter (&FastCtx.Ctx).
func AsCtx(c *domain.Ctx) *Ctx {
	return (*Ctx)(unsafe.Pointer(c))
}

// wrapHandlers converts facade handlers to domain handlers.
func wrapHandlers(mws []Handler) []domain.Handler {
	if len(mws) == 0 {
		return nil
	}
	out := make([]domain.Handler, len(mws))
	for i, mw := range mws {
		out[i] = wrapHandler(mw)
	}
	return out
}

// === Auth Types (re-exports) ===

// User represents an authenticatable user.
type User = domain.User

// TokenPair is the access+refresh pair returned on login.
type TokenPair = domain.TokenPair

// Claims represents JWT claims.
type Claims = domain.Claims

// Session represents a server-side session.
type Session = domain.Session

// DeviceInfo represents a registered mobile device.
type DeviceInfo = domain.DeviceInfo

// AuthConfig configures auth system behavior.
type AuthConfig = domain.AuthConfig

// OAuthConfig configures an OAuth2/OIDC provider.
type OAuthConfig = domain.OAuthConfig

// OAuthEndpoints are the OAuth2 provider endpoints.
type OAuthEndpoints = domain.OAuthEndpoints

// SessionStore manages server-side sessions.
type SessionStore = domain.SessionStore

// PasswordHasher hashes and verifies passwords.
type PasswordHasher = domain.PasswordHasher

// === Auth Adapters ===

// PasswordHasherAdapter returns a bcrypt password hasher.
func PasswordHasherAdapter(cost ...int) PasswordHasher {
	return bcryptadapter.New(cost...)
}

// JWTProvider returns a JWT TokenProvider (panics if JWTSecret is too short).
func JWTProvider(cfg AuthConfig) ports.TokenProvider {
	return jwtadapter.MustNew(cfg)
}

// MemorySessionStore creates an in-memory session store.
func MemorySessionStore() SessionStore {
	return sessionadapter.NewMemory()
}

// RedisSessionStore creates a Redis session store.
// The ttl parameter controls the maximum session lifetime.
func RedisSessionStore(client *redis.Client, ttl time.Duration) SessionStore {
	return sessionadapter.NewRedis(client, ttl)
}

// === Cache ===

// Cache is the Nitro L1/L2 cache interface (memory / Redis).
type Cache = domain.Cache

// MemoryCache creates an in-memory cache (domain.Cache).
func MemoryCache(maxSize int) Cache {
	return cacheadapter.NewMemoryCache(maxSize)
}

// RedisCacheOption configures the Redis L2 cache.
type RedisCacheOption = cacheadapter.RedisOption

// WithRedisKeyPrefix sets the Redis key prefix (default nitro:cache:).
func WithRedisKeyPrefix(prefix string) RedisCacheOption {
	return cacheadapter.WithKeyPrefix(prefix)
}

// WithRedisDefaultTTL sets the TTL used when Set receives ttl==0.
func WithRedisDefaultTTL(ttl time.Duration) RedisCacheOption {
	return cacheadapter.WithDefaultTTL(ttl)
}

// RedisCache creates a Redis L2 cache (JSON).
func RedisCache(client *redis.Client, opts ...RedisCacheOption) Cache {
	return cacheadapter.NewRedisCache(client, opts...)
}

// === OIDC / Keycloak ===

// OIDCConfig configures JWT validation via JWKS.
type OIDCConfig = oidcadapter.Config

// OIDCProvider creates a TokenProvider that validates RS256 JWTs via JWKS.
func OIDCProvider(cfg OIDCConfig) (ports.TokenProvider, error) {
	return oidcadapter.New(cfg)
}

// KeycloakConfig configures the Keycloak client (tokens + admin + JWKS).
type KeycloakConfig = keycloakadapter.Config

// KeycloakService groups Provider + Tokens + Admin.
type KeycloakService = keycloakadapter.Service

// KeycloakTokenResponse is the Login/Refresh response.
type KeycloakTokenResponse = keycloakadapter.TokenResponse

// KeycloakUser is the Admin API user representation.
type KeycloakUser = keycloakadapter.User

// KeycloakRole is a realm role.
type KeycloakRole = keycloakadapter.Role

// KeycloakCredential is a credential (password) in the Admin API.
type KeycloakCredential = keycloakadapter.Credential

// KeycloakClient manages login/refresh/logout (password grant).
type KeycloakClient = keycloakadapter.Client

// KeycloakAdmin manages users/roles via Admin REST.
type KeycloakAdmin = keycloakadapter.AdminClient

// NewKeycloak creates a Service with TokenProvider + Client + Admin.
func NewKeycloak(cfg KeycloakConfig) (*KeycloakService, error) {
	return keycloakadapter.New(cfg)
}

// KeycloakProvider creates only the TokenProvider (JWKS + refresh/logout).
func KeycloakProvider(cfg KeycloakConfig) (ports.TokenProvider, error) {
	return keycloakadapter.NewProvider(cfg)
}

// NewKeycloakClient creates the token client.
func NewKeycloakClient(cfg KeycloakConfig) KeycloakClient {
	return keycloakadapter.NewClient(cfg)
}

// NewKeycloakAdmin creates the Admin REST client.
func NewKeycloakAdmin(cfg KeycloakConfig) KeycloakAdmin {
	return keycloakadapter.NewAdmin(cfg)
}

// === Auth Middleware ===

// JWTMiddleware returns middleware that validates JWT Bearer tokens.
func JWTMiddleware(provider ports.TokenProvider) Handler {
	mw := nitroadapter.JWTMiddleware(provider)
	return wrapAdapter(mw)
}

// SessionMiddleware returns middleware that authenticates via session cookie.
func SessionMiddleware(store SessionStore, cookieName string) Handler {
	mw := nitroadapter.SessionMiddleware(store, cookieName)
	return wrapAdapter(mw)
}

// RequireRole returns middleware that checks user roles.
func RequireRole(roles ...string) Handler {
	mw := nitroadapter.RequireRole(roles...)
	return wrapAdapter(mw)
}

// RequireRoleOption configures RequireRoles (bypass, etc.).
type RequireRoleOption = nitroadapter.RequireRoleOption

// WithBypassRoles allows roles that pass without being in the required list.
func WithBypassRoles(roles ...string) RequireRoleOption {
	return nitroadapter.WithBypassRoles(roles...)
}

// RequireRoles is RequireRole with options (e.g. WithBypassRoles("super_admin")).
//
//	nitro.RequireRoles([]string{"admin"}, nitro.WithBypassRoles("super_admin"))
func RequireRoles(roles []string, opts ...RequireRoleOption) Handler {
	mw := nitroadapter.RequireRoles(roles, opts...)
	return wrapAdapter(mw)
}

// === Auth Use Cases ===

// AuthUseCase implements authentication business logic.
type AuthUseCase = usecases.AuthUseCase

// AuthDeps holds dependencies for auth use cases.
type AuthDeps = usecases.AuthDeps

// NewAuthUseCase creates a new auth use case.
func NewAuthUseCase(deps AuthDeps) *AuthUseCase {
	return usecases.NewAuthUseCase(deps)
}

// === OAuth2 Providers ===

// OAuthProvider is the OAuth2 provider interface.
type OAuthProvider = nitroadapter.OAuthProvider

// GoogleOAuth returns a Google OAuth2 provider.
func GoogleOAuth(cfg domain.OAuthConfig) OAuthProvider {
	return oauth2adapter.Google(cfg)
}

// GitHubOAuth returns a GitHub OAuth2 provider.
func GitHubOAuth(cfg domain.OAuthConfig) OAuthProvider {
	return oauth2adapter.GitHub(cfg)
}

// NewOAuthProvider creates a generic OAuth2 provider.
func NewOAuthProvider(name string, cfg domain.OAuthConfig, userFunc oauth2adapter.UserFunc) OAuthProvider {
	return oauth2adapter.New(name, cfg, userFunc)
}

// OAuthRedirectHandler returns a handler that redirects to the OAuth provider.
func OAuthRedirectHandler(provider OAuthProvider) Handler {
	mw := nitroadapter.OAuthRedirectHandler(provider)
	return wrapAdapter(mw)
}

// OAuthCallbackHandler returns a handler that processes the OAuth callback.
func OAuthCallbackHandler(provider OAuthProvider, auth *AuthUseCase) Handler {
	mw := nitroadapter.OAuthCallbackHandler(provider, auth)
	return wrapAdapter(mw)
}

// === App (principal entry point) ===

// App is the Nitro application.
type App struct {
	inner                     *nitroadapter.App
	sched                     *nitroscheduler.Scheduler
	cfg                       Config
	cache                     Cache
	sql                       any // e.g. *pgx.DB via pgx.Attach
	modelRepos                map[reflect.Type]func(table string) domain.ResourceRepository
	defaultsApplied           bool
	defaultResourceMiddleware []Handler
}

// Route represents a newly registered route.
type Route struct {
	inner *nitroadapter.Route
}

// Group represents a subset of routes with a shared prefix and middlewares.
type Group struct {
	app             *App
	inner           *nitroadapter.Group
	writeMiddleware []Handler
}

// New creates a new App instance.
//
// Enabled by default:
//   - Security headers + RequestID (adapter)
//   - Recover, Logger, MaxBody 1MiB (+ permissive CORS in EnvDevelopment)
//   - OpenTelemetry HTTP + global Tracer/Meter (consumed by the DB Gateway)
//   - Resource cache 5m (when registering Resource)
//   - Listen with graceful shutdown
//
// Env: Config.Env, else NITRO_ENV (development|dev|local), else production.
//
// Opt-out: Config.DisableDefaults, DisableRequestID, Security.DisableHeaders,
// DisableGracefulShutdown, DisableTelemetry, DisableInlineRecover.
// Env: NITRO_OTEL=0 / OTEL_SDK_DISABLED=true; NITRO_DB_QUIET=1 (DB only).
func New(cfg ...Config) *App {
	var c Config
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if c.Validator == nil {
		c.Validator = usecases.NewValidator()
	}
	// Convention: NITRO_ENV=development|dev|local selects EnvDevelopment when
	// the code does not set Config.Env. It can never tighten or loosen an
	// explicit choice, and an unset variable keeps the safe production default.
	if c.Env == EnvProduction && envFromProcess() == EnvDevelopment {
		c.Env = EnvDevelopment
	}
	if c.DisableTelemetry {
		oteladapter.ForceDisable()
	} else {
		if err := EnsureTelemetry(c.ServiceName); err != nil {
			log.Printf("nitro: telemetry: %v", err)
		}
	}
	app := &App{inner: nitroadapter.New(c), cfg: c, cache: c.Cache}
	if !c.DisableDefaults {
		app.applyDefaults()
	}
	if !c.DisableTelemetry && !oteladapter.Disabled() {
		app.Hooks().OnShutdown(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return oteladapter.Shutdown(ctx)
		})
	}
	if c.EnableLogger {
		app.Logger()
	}
	if c.EnablePrometheus {
		app.Prometheus(c.PrometheusPath)
	}
	return app
}

// SetCache sets the App default L2 cache (e.g. Redis). Resources without an explicit Cache
// inherit this cache + a path-derived CacheBustPattern and disable L1.
func (a *App) SetCache(c Cache) *App {
	a.cache = c
	a.cfg.Cache = c
	return a
}

// SetDefaultResourceMiddleware prepends handlers to every app.Resource that is
// not marked Public. Typical pattern after AuthRoutes:
//
//	app.SetDefaultResourceMiddleware(nitro.JWTMiddleware(auth.Provider()))
//
// Resources with ResourceConfig.Public=true skip these defaults (intentional
// open CRUD). Explicit Middleware/Read/Write on the Resource still apply and
// are appended after the defaults.
func (a *App) SetDefaultResourceMiddleware(handlers ...Handler) *App {
	if a == nil {
		return a
	}
	a.defaultResourceMiddleware = append([]Handler(nil), handlers...)
	return a
}

// DefaultResourceMiddleware returns a copy of the App-level Resource middleware.
func (a *App) DefaultResourceMiddleware() []Handler {
	if a == nil || len(a.defaultResourceMiddleware) == 0 {
		return nil
	}
	return append([]Handler(nil), a.defaultResourceMiddleware...)
}

// Cache returns the App default L2 cache (may be nil).
func (a *App) Cache() Cache {
	return a.cache
}

// SetSQL stores an opaque DB handle (e.g. pgx.Attach).
func (a *App) SetSQL(db any) *App {
	a.sql = db
	return a
}

// SQL returns the handle passed to SetSQL (may be nil).
func (a *App) SQL() any {
	return a.sql
}

// App returns the App that owns the group.
func (g *Group) App() *App {
	return g.app
}

// Bind associates a model type with a repository factory (e.g. Postgres Store).
// Prefer pgx.Attach + pgx.Resource[T] (lazy bind) in new apps.
//
//	pgx.Bind[Driver](app, db)
//	api.Resource("/drivers", &Driver{}, &nitro.ResourceConfig{Filterable: []string{"name"}})
func Bind[T any](a *App, factory func(table string) domain.ResourceRepository) {
	if a == nil || factory == nil {
		return
	}
	if a.modelRepos == nil {
		a.modelRepos = make(map[reflect.Type]func(table string) domain.ResourceRepository)
	}
	a.modelRepos[modelType[T]()] = factory
}

func modelType[T any]() reflect.Type {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Ptr {
		return t.Elem()
	}
	return t
}

func (a *App) repoForModel(model any, prefix string) domain.ResourceRepository {
	if a == nil || len(a.modelRepos) == 0 || model == nil {
		return nil
	}
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	factory, ok := a.modelRepos[t]
	if !ok || factory == nil {
		return nil
	}
	return factory(resourceTableName(prefix))
}

func resourceTableName(prefix string) string {
	p := strings.Trim(prefix, "/")
	if p == "" {
		return "items"
	}
	if i := strings.LastIndex(p, "/"); i >= 0 {
		p = p[i+1:]
	}
	return p
}

// Use registers a global middleware applied to all routes.

func envFromProcess() domain.Env {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NITRO_ENV"))) {
	case "development", "dev", "local":
		return domain.EnvDevelopment
	}
	return domain.EnvProduction
}
