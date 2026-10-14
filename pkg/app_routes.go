package nitro

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"reflect"

	nitroadapter "github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
	nitroscheduler "github.com/rodrigocelsius/go-nitro/scheduler/v2"
	"github.com/valyala/fasthttp"
)

func (a *App) Use(mw Handler) *App {
	a.inner.Use(wrapHandler(mw))
	return a
}

// UsePost registers a global middleware run AFTER the handler.
func (a *App) UsePost(mw Handler) *App {
	a.inner.UsePost(wrapHandler(mw))
	return a
}

// DomainHandler is the adapter-level handler signature (no FastCtx wrap).
type DomainHandler = domain.Handler

// Get registers a handler for GET requests.
func (a *App) Get(path string, handler Handler, mws ...Handler) *Route {
	inner := a.inner.Get(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// GetDomain registers a domain.Handler without wrapHandler (no extra indirection).
//
//	app.GetDomain("/", func(c *domain.Ctx) error {
//	    return nitro.AsCtx(c).Write(200, body)
//	})
func (a *App) GetDomain(path string, handler DomainHandler, mws ...DomainHandler) *Route {
	return &Route{inner: a.inner.Get(path, handler, mws...)}
}

// Post registers a handler for POST requests.
func (a *App) Post(path string, handler Handler, mws ...Handler) *Route {
	inner := a.inner.Post(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// PostDomain is GetDomain for POST.
func (a *App) PostDomain(path string, handler DomainHandler, mws ...DomainHandler) *Route {
	return &Route{inner: a.inner.Post(path, handler, mws...)}
}

// Put registers a handler for PUT requests.
func (a *App) Put(path string, handler Handler, mws ...Handler) *Route {
	inner := a.inner.Put(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// PutDomain is GetDomain for PUT.
func (a *App) PutDomain(path string, handler DomainHandler, mws ...DomainHandler) *Route {
	return &Route{inner: a.inner.Put(path, handler, mws...)}
}

// Delete registers a handler for DELETE requests.
func (a *App) Delete(path string, handler Handler, mws ...Handler) *Route {
	inner := a.inner.Delete(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// DeleteDomain is GetDomain for DELETE.
func (a *App) DeleteDomain(path string, handler DomainHandler, mws ...DomainHandler) *Route {
	return &Route{inner: a.inner.Delete(path, handler, mws...)}
}

// Patch registers a handler for PATCH requests.
func (a *App) Patch(path string, handler Handler, mws ...Handler) *Route {
	inner := a.inner.Patch(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// PatchDomain is GetDomain for PATCH.
func (a *App) PatchDomain(path string, handler DomainHandler, mws ...DomainHandler) *Route {
	return &Route{inner: a.inner.Patch(path, handler, mws...)}
}

// Head registers a handler for HEAD requests.
func (a *App) Head(path string, handler Handler, mws ...Handler) *Route {
	inner := a.inner.Head(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// Options registers a handler for OPTIONS requests.
func (a *App) Options(path string, handler Handler, mws ...Handler) *Route {
	inner := a.inner.Options(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// Group creates a subgroup with a shared prefix and middlewares.
// Prefixes accumulate: app.Group("/api").Group("/v1") → "/api/v1"
func (a *App) Group(prefix string, mws ...Handler) *Group {
	inner := a.inner.Group(prefix, wrapHandlers(mws)...)
	return &Group{app: a, inner: inner}
}

// Static serves static files. Use StaticConfig{SinglePageApp: true} for SPA.
func (a *App) Static(prefix, root string, cfg ...StaticConfig) *Route {
	return &Route{inner: a.inner.Static(prefix, root, cfg...)}
}

// Mount mounts a sub-App under the prefix (sub-app middlewares and routes preserved).
//
//	v1 := nitro.New()
//	v1.Get("/users", list)
//	app.Mount("/api/v1", v1)
func (a *App) Mount(prefix string, sub *App) *App {
	if sub != nil {
		a.inner.Mount(prefix, sub.inner)
	}
	return a
}

// Hooks returns the lifecycle hooks registrar.
//
//	app.Hooks().OnListen(func(d nitro.ListenData) error { ... }).
//		OnShutdown(func() error { ... })
func (a *App) Hooks() *Hooks {
	return a.inner.Hooks()
}

// GetRouteURL reconstructs the path of a named route.
//
//	app.Get("/users/:id", h).Name("user.show")
//	app.GetRouteURL("user.show", nitro.Map{"id": 42})
func (a *App) GetRouteURL(name string, params ...Map) string {
	return a.inner.GetRouteURL(name, params...)
}

// Name assigns an explicit name to the route.
func (r *Route) Name(name string) *Route {
	r.inner.Name(name)
	return r
}

// === Group Methods ===

// Get registers a handler for GET requests on the group.
func (g *Group) Get(path string, handler Handler, mws ...Handler) *Route {
	inner := g.inner.Get(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// Post registers a handler for POST requests on the group.
func (g *Group) Post(path string, handler Handler, mws ...Handler) *Route {
	inner := g.inner.Post(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// Put registers a handler for PUT requests on the group.
func (g *Group) Put(path string, handler Handler, mws ...Handler) *Route {
	inner := g.inner.Put(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// Delete registers a handler for DELETE requests on the group.
func (g *Group) Delete(path string, handler Handler, mws ...Handler) *Route {
	inner := g.inner.Delete(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// Patch registers a handler for PATCH requests on the group.
func (g *Group) Patch(path string, handler Handler, mws ...Handler) *Route {
	inner := g.inner.Patch(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// Head registers a handler for HEAD requests on the group.
func (g *Group) Head(path string, handler Handler, mws ...Handler) *Route {
	inner := g.inner.Head(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// Options registers a handler for OPTIONS requests on the group.
func (g *Group) Options(path string, handler Handler, mws ...Handler) *Route {
	inner := g.inner.Options(path, wrapHandler(handler), wrapHandlers(mws)...)
	return &Route{inner: inner}
}

// Use adds middlewares to the group.
func (g *Group) Use(mws ...Handler) *Group {
	inner := g.inner.Use(wrapHandlers(mws)...)
	return &Group{app: g.app, inner: inner, writeMiddleware: append([]Handler{}, g.writeMiddleware...)}
}

// UseWrite sets middlewares applied only on Resource writes (POST/PATCH/PUT/DELETE).
// Fiber-style: declare once on the group instead of on each Resource.
//
//	api := app.Group("/api", jwt)
//	api.UseWrite(nitro.RequireRoles([]string{"admin"}, nitro.WithBypassRoles("super_admin")))
//	api.Resource("/drivers", &Driver{}, repo)
func (g *Group) UseWrite(mws ...Handler) *Group {
	g.writeMiddleware = append(g.writeMiddleware, mws...)
	return g
}

// Group creates a subgroup inside an existing group.
func (g *Group) Group(prefix string, mws ...Handler) *Group {
	inner := g.inner.Group(prefix, wrapHandlers(mws)...)
	return &Group{
		app:             g.app,
		inner:           inner,
		writeMiddleware: append([]Handler{}, g.writeMiddleware...),
	}
}

// Prefix returns the accumulated path prefix.
func (g *Group) Prefix() string {
	return g.inner.Prefix()
}

// Resource registers CRUD under the group prefix (inherits middlewares / UseWrite).
//
//	api.Resource("/drivers", db, &Driver{}, cfg)
//	api.Resource(&Product{}, store) // → {group}/products
func (g *Group) Resource(first any, args ...any) *Group {
	path, rest := splitResourceFirst(first, args)
	// Infer path early when omitted so group prefix joins correctly.
	if path == "" {
		model, _ := resolveResourceArgs(g.app, "", rest)
		if model != nil {
			path = "/" + resourcePathFromModel(model)
		}
	}
	prefix := joinResourcePath(g.inner.Prefix(), path)
	rest = prependGroupMiddleware(rest, unwrapGroupHandlers(g.inner.Middlewares()))
	rest = prependGroupWriteMiddleware(rest, g.writeMiddleware)
	g.app.Resource(prefix, rest...)
	return g
}

// resolveResourceArgs extracts model, db (ResourceDB), repo, and config from args.
func resolveResourceArgs(app *App, prefix string, args []any) (model any, out []any) {
	var sqlDB domain.ResourceDB
	var repo domain.ResourceRepository
	var extra []any

	if app != nil {
		if d, ok := app.SQL().(domain.ResourceDB); ok {
			sqlDB = d
		}
	}

	for _, a := range args {
		switch v := a.(type) {
		case domain.ResourceRepository:
			repo = v
		case domain.AsResourceRepository:
			repo = v.AsResourceRepository()
		case domain.ResourceDB:
			sqlDB = v
		case *ResourceConfig, ResourceConfig:
			extra = append(extra, a)
		case nil:
			continue
		default:
			if model == nil && isResourceModel(v) {
				model = v
				continue
			}
			extra = append(extra, a) // reported by Resource as a config error
		}
	}

	if repo == nil && sqlDB != nil && model != nil {
		repo = sqlDB.ResourceRepository(model, resourceTableName(prefix))
	}
	if repo != nil {
		out = append(out, repo)
	}
	out = append(out, extra...)
	return model, out
}

func isResourceModel(v any) bool {
	t := reflect.TypeOf(v)
	if t == nil {
		return false
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}

func joinResourcePath(prefix, path string) string {
	if path == "" {
		return prefix
	}
	if prefix == "" {
		return path
	}
	if prefix[len(prefix)-1] == '/' && path[0] == '/' {
		return prefix + path[1:]
	}
	if prefix[len(prefix)-1] != '/' && path[0] != '/' {
		return prefix + "/" + path
	}
	return prefix + path
}

func unwrapGroupHandlers(mws []domain.Handler) []Handler {
	if len(mws) == 0 {
		return nil
	}
	out := make([]Handler, len(mws))
	for i, mw := range mws {
		out[i] = wrapAdapter(mw)
	}
	return out
}

func prependGroupMiddleware(args []any, groupMWs []Handler) []any {
	if len(groupMWs) == 0 {
		return args
	}
	var cfg *ResourceConfig
	out := make([]any, 0, len(args)+1)
	for _, arg := range args {
		switch v := arg.(type) {
		case *ResourceConfig:
			cp := *v
			cp.Middleware = append(append([]Handler{}, groupMWs...), v.Middleware...)
			cfg = &cp
			out = append(out, cfg)
		case ResourceConfig:
			cp := v
			cp.Middleware = append(append([]Handler{}, groupMWs...), v.Middleware...)
			cfg = &cp
			out = append(out, cfg)
		default:
			out = append(out, arg)
		}
	}
	if cfg == nil {
		out = append(out, &ResourceConfig{Middleware: append([]Handler{}, groupMWs...)})
	}
	return out
}

// prependGroupWriteMiddleware merges group UseWrite into ResourceConfig.WriteMiddleware
// (group first; Resource may append more).
func prependGroupWriteMiddleware(args []any, writeMWs []Handler) []any {
	if len(writeMWs) == 0 {
		return args
	}
	var cfg *ResourceConfig
	out := make([]any, 0, len(args)+1)
	for _, arg := range args {
		switch v := arg.(type) {
		case *ResourceConfig:
			cp := *v
			cp.WriteMiddleware = append(append([]Handler{}, writeMWs...), v.WriteMiddleware...)
			cfg = &cp
			out = append(out, cfg)
		case ResourceConfig:
			cp := v
			cp.WriteMiddleware = append(append([]Handler{}, writeMWs...), v.WriteMiddleware...)
			cfg = &cp
			out = append(out, cfg)
		default:
			out = append(out, arg)
		}
	}
	if cfg == nil {
		out = append(out, &ResourceConfig{WriteMiddleware: append([]Handler{}, writeMWs...)})
	}
	return out
}

// === Routes ===
func (a *App) Routes() []RouteInfo {
	return a.inner.Routes()
}

// Handler returns the function that processes fasthttp requests.
// Exposed for in-process tests and composition with other servers.
func (a *App) Handler() func(fctx *fasthttp.RequestCtx) {
	return a.inner.Handler()
}

// Serve starts the server on the listener with fasthttp's optimized config
// (Concurrency, NoDefaultDate, etc.). Prefer this over a raw Server with Handler().
func (a *App) Serve(ln net.Listener) error {
	return a.inner.Serve(ln)
}

// FastHTTPServer returns the configured *fasthttp.Server (tests/benches).
func (a *App) FastHTTPServer() *fasthttp.Server {
	return a.inner.FastHTTPServer()
}

// Schedule registers a periodic task (cron or interval).
// The scheduler starts automatically with Listen

// and stops on shutdown.
//
//	app.Schedule("*/5 * * * *", func(ctx context.Context) error {
//	    return cleanup(ctx)
//	})
//	app.Schedule("1h", func(ctx context.Context) error {
//	    return report(ctx)
//	})
func (a *App) Schedule(expr string, fn func(ctx context.Context) error) error {
	if a.sched == nil {
		a.sched = nitroscheduler.New()
	}
	_, err := a.sched.Every(expr, fn)
	return err
}

// ScheduleNamed registers a task with an explicit name.
func (a *App) ScheduleNamed(name, expr string, fn func(ctx context.Context) error) error {
	if a.sched == nil {
		a.sched = nitroscheduler.New()
	}
	_, err := a.sched.ScheduleWithName(name, expr, fn)
	return err
}

func (a *App) startScheduler() error {
	if a.sched == nil {
		return nil
	}
	return a.sched.Start(context.Background())
}

func (a *App) stopScheduler() {
	if a.sched != nil {
		_ = a.sched.Stop()
	}
}

// Test runs an in-process HTTP request without starting a server.
func (a *App) Test(method, path string, body io.Reader) (*fasthttp.Response, error) {
	return a.inner.Test(method, path, body)
}

// TestWithHeaders is like Test with extra request headers.
func (a *App) TestWithHeaders(method, path string, body io.Reader, headers map[string]string) (*fasthttp.Response, error) {
	return a.inner.TestWithHeaders(method, path, body, headers)
}

// Configuration contract: call setters (Use, Defaults, Logger, SetCache, SetSQL,
// EnableTLS, OpenAPI/Swagger, schedule helpers, Bind, Resource, route methods)
// before Listen / ListenTLS / ListenAutoTLS. After Listen returns (or the process
// blocks), further registration is undefined.
//
// Listen starts the HTTP server on the given address.
// By default it uses graceful shutdown (SIGINT/SIGTERM, 30s timeout).
// Disable with Config.DisableGracefulShutdown or customize via cfg.
//
//	app.Listen(":8080")
//	app.Listen(":8080", GracefulShutdownConfig{Timeout: 60*time.Second})
func (a *App) Listen(addr string, cfg ...GracefulShutdownConfig) error {
	if a.cfg.DisableGracefulShutdown {
		if err := a.startScheduler(); err != nil {
			return err
		}
		defer a.stopScheduler()
		return a.inner.Listen(addr)
	}
	return a.listenGraceful(addr, cfg...)
}

// ListenTLS starts the HTTPS server with the given certificate and key.
// For embedded certificates, use ListenTLS(addr, "", "") and configure
// TLS via Config.TLSConfig or EnableTLS().
// Graceful shutdown is enabled by default (same opt-out as Listen).
func (a *App) ListenTLS(addr, certFile, keyFile string, cfg ...GracefulShutdownConfig) error {
	if a.cfg.DisableGracefulShutdown {
		if err := a.startScheduler(); err != nil {
			return err
		}
		defer a.stopScheduler()
		return a.inner.ListenTLS(addr, certFile, keyFile)
	}
	return a.listenGracefulTLS(addr, certFile, keyFile, cfg...)
}

// ListenAutoTLS starts the HTTPS server with automatic certificates
// via Let's Encrypt (ACME). The given domains determine certificate
// coverage. Graceful shutdown is enabled by default.
func (a *App) ListenAutoTLS(addr string, domains ...string) error {
	if a.cfg.DisableGracefulShutdown {
		if err := a.startScheduler(); err != nil {
			return err
		}
		defer a.stopScheduler()
		return a.inner.ListenAutoTLS(addr, domains...)
	}
	return a.listenGracefulAutoTLS(addr, domains)
}

func (a *App) listenGraceful(addr string, cfg ...GracefulShutdownConfig) error {
	if err := a.startScheduler(); err != nil {
		return err
	}
	config := a.wrapGracefulConfig(cfg...)
	return a.inner.ListenWithGracefulShutdown(addr, config)
}

func (a *App) listenGracefulTLS(addr, certFile, keyFile string, cfg ...GracefulShutdownConfig) error {
	if err := a.startScheduler(); err != nil {
		return err
	}
	config := a.wrapGracefulConfig(cfg...)
	return a.inner.ListenWithGracefulShutdownTLS(addr, certFile, keyFile, config)
}

func (a *App) listenGracefulAutoTLS(addr string, domains []string, cfg ...GracefulShutdownConfig) error {
	if err := a.startScheduler(); err != nil {
		return err
	}
	config := a.wrapGracefulConfig(cfg...)
	return a.inner.ListenWithGracefulShutdownAutoTLS(addr, domains, config)
}

func (a *App) wrapGracefulConfig(cfg ...GracefulShutdownConfig) GracefulShutdownConfig {
	var config GracefulShutdownConfig
	if len(cfg) > 0 {
		config = cfg[0]
	}
	userOnShutdown := config.OnShutdown
	config.OnShutdown = func() {
		a.stopScheduler()
		if userOnShutdown != nil {
			userOnShutdown()
		}
	}
	return config
}

// EnableTLS applies an explicit TLS configuration to the server.
// Useful for programmatic configuration before Listen().
func (a *App) EnableTLS(tlsConfig *tls.Config) *App {
	a.inner.EnableTLS(tlsConfig)
	return a
}

// OpenAPI automatically registers GET /openapi.json
// with the spec generated from registered routes.
// Call AFTER mounting routes (Resource, auth, etc.).
func (a *App) OpenAPI(title, description, version string) *App {
	nitroadapter.RegisterOpenAPIFromRoutes(a.inner, title, description, version)
	return a
}

// Swagger generates OpenAPI from routes and serves Swagger UI.
//
//	app.Swagger("Transport API", "Transport API", "0.1.0")
//	// GET /openapi.json
//	// GET /docs
//
// Call AFTER registering all routes.
func (a *App) Swagger(title, description, version string) *App {
	nitroadapter.RegisterSwagger(a.inner, title, description, version)
	return a
}

// Shutdown shuts the server down gracefully.
func (a *App) Shutdown() error {
	return a.inner.Shutdown()
}

// NamedHealth adapts a named function to HealthChecker.
//
//	app.HealthCheck(nitro.NamedHealth("redis", func() error { return rdb.Ping(ctx).Err() }))
func NamedHealth(name string, fn func() error) domain.HealthChecker {
	return &namedHealth{name: name, fn: fn}
}

type namedHealth struct {
	name string
	fn   func() error
}

func (n *namedHealth) Name() string { return n.name }
func (n *namedHealth) Check() error { return n.fn() }

// Err returns the configuration mistakes recorded while building the app
// (insecure Resource setup, bad arguments, …). Listen*/Test return it too, so
// checking it is optional; use it to fail fast in your own bootstrap code.
func (a *App) Err() error { return a.inner.Err() }

// fail records a configuration error instead of panicking mid-chain.
func (a *App) fail(err error) *App {
	a.inner.AddConfigError(err)
	return a
}

// HealthCheck registers a health checker.
// On the first checker, /health, /ready, and /live routes are created automatically.
// Accepts domain.HealthChecker or func() error.
func (a *App) HealthCheck(checker any) *App {
	switch v := checker.(type) {
	case domain.HealthChecker:
		a.inner.HealthCheck(v)
	case func() error:
		a.inner.HealthCheck(&funcHealthChecker{fn: v})
	default:
		return a.fail(errors.New("nitro: HealthCheck expects domain.HealthChecker or func() error"))
	}
	return a
}

// funcHealthChecker adapts func() error to domain.HealthChecker.
type funcHealthChecker struct {
	fn func() error
}

func (f *funcHealthChecker) Name() string { return "health" }
func (f *funcHealthChecker) Check() error { return f.fn() }

// HealthRoutes registers the /health, /ready, /live routes.
func (a *App) HealthRoutes() *App {
	a.inner.RegisterHealthRoutes()
	return a
}
