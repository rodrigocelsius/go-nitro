package fasthttp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
	"github.com/valyala/fasthttp"
	"golang.org/x/crypto/acme/autocert"
)

// App represents the Nitro application. It is the entry point to register
// routes and start the server. Implements ports.Server.
type App struct {
	routes     []domain.LinearRoute
	router     *RadixRouter
	routeNames map[string]string // "METHOD path" → name
	namedPaths map[string]string // name → path (reverse routing)

	globalPre  []domain.Handler
	globalPost []domain.Handler

	healthCheckers []domain.HealthChecker

	useLinear bool
	pool      sync.Pool
	server    *fasthttp.Server
	srvMu     sync.RWMutex // guards server (Listen/Serve vs Shutdown run on different goroutines)

	trusted *trustSet // per-app Config.TrustedProxies (nil → process-wide default)

	cfgMu   sync.Mutex
	cfgErrs []error // configuration mistakes collected while building (see Err)

	metrics *MetricsCollector
	hooks   *Hooks

	requestTimeout        time.Duration
	tlsConfig             *tls.Config
	errorHandler          func(c *domain.Ctx, err error) error
	validator             domain.Validator
	disableStartupMessage bool
	disableInlineRecover  bool
	env                   domain.Env
	maxRequestBodySize    int
	debug                 bool
}

// DefaultMaxRequestBodySize matches fasthttp's own built-in default (4 MiB)
// so behavior is unchanged unless Config.MaxRequestBodySize is set
// explicitly. See vuln.md (V-15).
const DefaultMaxRequestBodySize = fasthttp.DefaultMaxRequestBodySize

// Route represents a newly registered route, allowing chained extra
// configuration (such as Name) without losing the ability to keep
// chaining calls on *App.
type Route struct {
	*App
	method string
	path   string
}

// New creates a new App instance.
func New(cfg ...domain.Config) *App {
	app := &App{
		router:     NewRadixRouter(),
		routeNames: make(map[string]string),
		namedPaths: make(map[string]string),
		hooks:      &Hooks{},
	}
	var config domain.Config
	if len(cfg) > 0 {
		config = cfg[0]
		app.useLinear = config.UseLinearRouter
		app.requestTimeout = config.RequestTimeout
		app.tlsConfig = config.TLSConfig
		app.errorHandler = config.ErrorHandler
		app.validator = config.Validator
		app.disableStartupMessage = config.DisableStartupMessage
		app.disableInlineRecover = config.DisableInlineRecover
		app.env = config.Env
		app.maxRequestBodySize = config.MaxRequestBodySize
		app.debug = config.Debug
		if config.TrustedProxies != nil {
			ts, err := parseTrust(config.TrustedProxies)
			if err != nil {
				app.AddConfigError(fmt.Errorf("nitro: Config.TrustedProxies: %w", err))
			} else {
				app.trusted = ts
			}
		}
	}
	if app.validator == nil {
		app.validator = usecases.NewValidator()
	}
	app.pool.New = func() any {
		return NewFastCtx()
	}
	app.applySecurityDefaults(config)
	if !config.DisableRequestID {
		app.globalPre = append(app.globalPre, RequestIDMiddleware())
	}
	return app
}

// SetValidator sets the validator used by BodyJSON/Validate on this App.
func (a *App) SetValidator(v domain.Validator) {
	if v == nil {
		v = usecases.NewValidator()
	}
	a.validator = v
}

// Validator returns the App validator.
func (a *App) Validator() domain.Validator {
	if a.validator == nil {
		a.validator = usecases.NewValidator()
	}
	return a.validator
}

// Use registers a global middleware applied to all routes.
func (a *App) Use(mw domain.Handler) *App {
	a.globalPre = append(a.globalPre, mw)
	return a
}

// UsePost registers a global middleware run AFTER the handler.
func (a *App) UsePost(mw domain.Handler) *App {
	a.globalPost = append(a.globalPost, mw)
	return a
}

// Name assigns an explicit name to the route (reverse routing via GetRouteURL / URL).
// Fires OnRoute again with RouteInfo.Name populated.
func (r *Route) Name(name string) *Route {
	r.App.routeNames[r.method+" "+r.path] = name
	r.App.namedPaths[name] = r.path
	r.App.fireOnRoute(domain.RouteInfo{Method: r.method, Path: r.path, Name: name})
	return r
}

// Get registers a handler for GET requests.
func (a *App) Get(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return a.add("GET", path, handler, mws)
}

// Post registers a handler for POST requests.
func (a *App) Post(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return a.add("POST", path, handler, mws)
}

// Put registers a handler for PUT requests.
func (a *App) Put(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return a.add("PUT", path, handler, mws)
}

// Delete registers a handler for DELETE requests.
func (a *App) Delete(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return a.add("DELETE", path, handler, mws)
}

// Patch registers a handler for PATCH requests.
func (a *App) Patch(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return a.add("PATCH", path, handler, mws)
}

// Head registers a handler for HEAD requests.
func (a *App) Head(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return a.add("HEAD", path, handler, mws)
}

// Options registers a handler for OPTIONS requests.
func (a *App) Options(path string, handler domain.Handler, mws ...domain.Handler) *Route {
	return a.add("OPTIONS", path, handler, mws)
}

func (a *App) add(method, path string, handler domain.Handler, routeMws []domain.Handler) *Route {
	// Wrap middlewares with timing if metrics enabled
	globalPre := a.globalPre
	if a.metrics != nil && a.metrics.enabled {
		wrapped := make([]domain.Handler, len(globalPre))
		for i, mw := range globalPre {
			wrapped[i] = a.metrics.Wrap("global["+strconv.Itoa(i)+"]", mw)
		}
		globalPre = wrapped

		wrappedRoute := make([]domain.Handler, len(routeMws))
		for i, mw := range routeMws {
			wrappedRoute[i] = a.metrics.Wrap("route["+strconv.Itoa(i)+"]", mw)
		}
		routeMws = wrappedRoute
	}

	compiled := usecases.CompileHandler(handler, globalPre, a.globalPost, routeMws)
	var route *Route
	if a.useLinear {
		a.routes = append(a.routes, domain.LinearRoute{Method: method, Path: path, Handler: compiled})
		route = &Route{App: a, method: method, path: path}
	} else {
		a.router.Add(method, path, compiled)
		route = &Route{App: a, method: method, path: path}
	}
	a.fireOnRoute(domain.RouteInfo{Method: method, Path: path})
	return route
}

// Routes returns all routes registered on this App.
func (a *App) Routes() []domain.RouteInfo {
	routes := a.router.Routes()
	for i := range routes {
		key := routes[i].Method + " " + routes[i].Path
		routes[i].Name = a.routeNames[key]
	}
	return routes
}

// Handler returns the function that processes requests.
func (a *App) Handler() func(fctx *fasthttp.RequestCtx) {
	noop := func(c *domain.Ctx) error { return nil }
	optionsHandler := usecases.CompileHandler(noop, a.globalPre, a.globalPost, nil)
	notFoundHandler := usecases.CompileHandler(func(c *domain.Ctx) error {
		return domain.ErrNotFound
	}, a.globalPre, a.globalPost, nil)
	methodNotAllowedHandler := usecases.CompileHandler(func(c *domain.Ctx) error {
		return domain.ErrMethodNotAllowed
	}, a.globalPre, a.globalPost, nil)
	metricsEnabled := a.metrics != nil && a.metrics.enabled
	panicBody := []byte(`{"error":"internal server error","message":"an unexpected error occurred"}`)

	if a.requestTimeout > 0 {
		return a.handlerWithTimeout(optionsHandler, notFoundHandler, methodNotAllowedHandler, metricsEnabled, panicBody)
	}
	if !metricsEnabled && a.disableInlineRecover {
		return a.handlerHot(optionsHandler, notFoundHandler, methodNotAllowedHandler)
	}
	return a.handlerFast(optionsHandler, notFoundHandler, methodNotAllowedHandler, metricsEnabled, panicBody)
}

// handlerHot is the lean fasthttp path: no metrics, no inline recover (pool still deferred).
func (a *App) handlerHot(
	optionsHandler, notFoundHandler, methodNotAllowedHandler domain.Handler,
) func(fctx *fasthttp.RequestCtx) {
	validator := a.validator
	return func(fctx *fasthttp.RequestCtx) {
		c := a.pool.Get().(*FastCtx)
		c.Reset(fctx)
		c.app = a
		c.validator = validator
		defer func() {
			c.app = nil
			a.pool.Put(c)
		}()

		h, ok := a.lookupHandler(fctx, c, optionsHandler)
		if !ok {
			h = a.missHandler(fctx, notFoundHandler, methodNotAllowedHandler)
		}
		if err := h(&c.Ctx); err != nil {
			a.handleError(&c.Ctx, err)
		}
	}
}

func (a *App) handlerFast(
	optionsHandler, notFoundHandler, methodNotAllowedHandler domain.Handler,
	metricsEnabled bool,
	panicBody []byte,
) func(fctx *fasthttp.RequestCtx) {
	validator := a.validator
	recoverInline := !a.disableInlineRecover
	return func(fctx *fasthttp.RequestCtx) {
		var start time.Time
		if metricsEnabled {
			start = time.Now()
		}

		c := a.pool.Get().(*FastCtx)
		c.Reset(fctx)
		c.app = a
		c.validator = validator
		defer func() {
			if recoverInline {
				if r := recover(); r != nil {
					fctx.Response.Reset()
					fctx.SetStatusCode(fasthttp.StatusInternalServerError)
					fctx.Response.Header.SetContentTypeBytes(contentTypeJSON)
					fctx.SetBody(a.recoverBody(r, panicBody))
				}
			}
			c.app = nil
			a.pool.Put(c)
		}()

		h, ok := a.lookupHandler(fctx, c, optionsHandler)
		if !ok {
			h = a.missHandler(fctx, notFoundHandler, methodNotAllowedHandler)
		}

		if err := h(&c.Ctx); err != nil {
			a.handleError(&c.Ctx, err)
		}
		if metricsEnabled {
			a.metrics.RecordRequest(fctx.Response.StatusCode(), time.Since(start))
		}
	}
}

// recoverBody builds the response body for a recovered panic. This is the
// *only* place a panic is actually caught (see recover.go's doc comment on
// RecoverMiddlewareWithDebug — pre-middleware registered via Use never
// observes a panic, since it unwinds straight past the compiled
// pre/handler/post chain up to this inline defer/recover). Because of
// that, the sanitized-stack-trace debug behavior must be gated by
// a.debug (Config.Debug) right here, not in the middleware, or V-33's
// intended protection (never leak internals unless explicitly opted in)
// would silently not apply.
func (a *App) recoverBody(r any, panicBody []byte) []byte {
	if !a.debug {
		return panicBody
	}
	sanitized := sanitizeStackTrace(string(debug.Stack()))
	return []byte(fmt.Sprintf(`{"error":"internal server error","message":%q,"stack":%q}`, fmt.Sprint(r), sanitized))
}

func (a *App) handlerWithTimeout(
	optionsHandler, notFoundHandler, methodNotAllowedHandler domain.Handler,
	metricsEnabled bool,
	panicBody []byte,
) func(fctx *fasthttp.RequestCtx) {
	timeout := a.requestTimeout
	validator := a.validator
	recoverInline := !a.disableInlineRecover
	return func(fctx *fasthttp.RequestCtx) {
		var start time.Time
		if metricsEnabled {
			start = time.Now()
		}

		c := a.pool.Get().(*FastCtx)
		c.Reset(fctx)
		c.app = a
		c.validator = validator
		defer func() {
			if recoverInline {
				if r := recover(); r != nil {
					fctx.Response.Reset()
					fctx.SetStatusCode(fasthttp.StatusInternalServerError)
					fctx.Response.Header.SetContentTypeBytes(contentTypeJSON)
					fctx.SetBody(a.recoverBody(r, panicBody))
				}
			}
			c.app = nil
			a.pool.Put(c)
		}()

		h, ok := a.lookupHandler(fctx, c, optionsHandler)
		if !ok {
			h = a.missHandler(fctx, notFoundHandler, methodNotAllowedHandler)
		}

		timeoutCtx, cancel := context.WithTimeout(c.RequestContext(), timeout)
		c.SetRequestContext(timeoutCtx)
		if err := h(&c.Ctx); err != nil {
			a.handleError(&c.Ctx, err)
		}
		cancel()

		if timeoutCtx.Err() == context.DeadlineExceeded && fctx.Response.StatusCode() == 200 {
			fctx.Response.Reset()
			fctx.SetStatusCode(fasthttp.StatusRequestTimeout)
			fctx.Response.Header.SetContentTypeBytes(contentTypeJSON)
			fctx.SetBodyString(`{"error":"request timeout","message":"request exceeded the time limit"}`)
		}
		if metricsEnabled {
			a.metrics.RecordRequest(fctx.Response.StatusCode(), time.Since(start))
		}
	}
}

// missHandler answers a lookup miss: 405 + Allow when the path exists under
// other methods (RFC 9110 §15.5.6), otherwise 404.
func (a *App) missHandler(fctx *fasthttp.RequestCtx, notFound, methodNotAllowed domain.Handler) domain.Handler {
	var allowed []string
	if a.useLinear {
		path := b2s(fctx.Path())
		for _, r := range a.routes {
			var scratch domain.Ctx
			if usecases.MatchPathLinear(r.Path, path, &scratch) && !slices.Contains(allowed, r.Method) {
				allowed = append(allowed, r.Method)
			}
		}
	} else {
		allowed = a.router.AllowedMethods(fctx.Path())
	}
	if len(allowed) == 0 {
		return notFound
	}
	if !slices.Contains(allowed, "OPTIONS") {
		allowed = append(allowed, "OPTIONS") // answered by the router for every path
	}
	slices.Sort(allowed)
	fctx.Response.Header.Set("Allow", strings.Join(allowed, ", "))
	return methodNotAllowed
}

func (a *App) lookupHandler(fctx *fasthttp.RequestCtx, c *FastCtx, optionsHandler domain.Handler) (domain.Handler, bool) {
	var h domain.Handler
	var ok bool
	if a.useLinear {
		h, ok = usecases.MatchLinear(a.routes, b2s(fctx.Method()), b2s(fctx.Path()), &c.Ctx)
	} else {
		h, ok = a.router.Lookup(fctx.Method(), fctx.Path(), &c.Ctx)
	}
	if ok {
		return h, true
	}
	if methodBytes(fctx.Method(), "OPTIONS") {
		return optionsHandler, true
	}
	return nil, false
}

// AddConfigError records a configuration mistake (insecure or invalid setup)
// without panicking in the middle of a fluent chain. Listen*/Serve refuse to
// start, and Test/Handler report it, while Err returns all of them.
func (a *App) AddConfigError(err error) {
	if err == nil {
		return
	}
	a.cfgMu.Lock()
	a.cfgErrs = append(a.cfgErrs, err)
	a.cfgMu.Unlock()
}

// Err returns the configuration errors recorded so far (nil when the app is
// correctly configured). Check it after building, or let Listen return it.
func (a *App) Err() error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	return errors.Join(a.cfgErrs...)
}

// DefaultConcurrency matches fasthttp / Fiber (256 * 1024).
const DefaultConcurrency = 256 * 1024

// serverConfig returns a *fasthttp.Server tuned for high concurrency.
// Concurrency follows the fasthttp/Fiber default (256*1024). NoDefault* skips
// headers generated on the hot path. Timeouts protect against hung connections.
func (a *App) serverConfig() *fasthttp.Server {
	maxBody := a.maxRequestBodySize
	if maxBody <= 0 {
		maxBody = DefaultMaxRequestBodySize
	}
	return &fasthttp.Server{
		Handler:            a.Handler(),
		ReadBufferSize:     4096,
		WriteBufferSize:    4096,
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		IdleTimeout:        120 * time.Second,
		MaxConnsPerIP:      0,
		MaxRequestsPerConn: 0,
		MaxRequestBodySize: maxBody,
		// Header names stay normalized: HTTP field names are case-insensitive
		// (RFC 9110 §5.1) and HTTP/2 proxies, Node fetch and browsers send
		// them lowercase. With normalization disabled, Peek("Authorization")
		// missed "authorization" and every JWT request from a BFF was 401.
		NoDefaultServerHeader: true,
		NoDefaultDate:         true,
		NoDefaultContentType:  true,
		Concurrency:           DefaultConcurrency,
	}
}

// newServer builds the fasthttp server and publishes it for Shutdown.
// Access to a.server is serialized: Listen runs on one goroutine while
// Shutdown is normally called from another (signal handler).
func (a *App) newServer() *fasthttp.Server {
	srv := a.serverConfig()
	a.srvMu.Lock()
	a.server = srv
	a.srvMu.Unlock()
	return srv
}

// currentServer returns the running server (nil before Listen/Serve).
func (a *App) currentServer() *fasthttp.Server {
	a.srvMu.RLock()
	defer a.srvMu.RUnlock()
	return a.server
}

// Serve starts serving on ln with the optimized server config.
// Prefer this over constructing a bare fasthttp.Server{Handler: app.Handler()}.
func (a *App) Serve(ln net.Listener) error {
	if err := a.Err(); err != nil {
		return err
	}
	return a.newServer().Serve(ln)
}

// FastHTTPServer returns a configured *fasthttp.Server (for tests/benches).
func (a *App) FastHTTPServer() *fasthttp.Server {
	return a.newServer()
}

// Listen starts the HTTP server on the given address.
func (a *App) Listen(addr string) error {
	if err := a.Err(); err != nil {
		return err
	}
	if err := a.fireOnListen(addr); err != nil {
		return err
	}
	srv := a.newServer()
	if a.tlsConfig != nil {
		srv.TLSConfig = a.tlsConfig
		a.printStartupBanner(addr, true)
		return srv.ListenAndServeTLS(addr, "", "")
	}
	warnPlainHTTPInProduction(a.env)
	a.printStartupBanner(addr, false)
	return srv.ListenAndServe(addr)
}

// warnPlainHTTPInProduction prints a stderr warning when the app boots
// with Env == EnvProduction over plain HTTP (no TLSConfig, cert/key, or
// autocert). This is not necessarily wrong — TLS is commonly terminated
// by an external proxy/load balancer — but the framework has no way to
// tell that apart from an operator simply forgetting to configure TLS,
// so a visible warning at boot is cheap insurance either way.
func warnPlainHTTPInProduction(env domain.Env) {
	if env == domain.EnvProduction {
		fmt.Fprintln(os.Stderr, "nitro: warning: Listen() serving plain HTTP with Env=production; if TLS is not terminated by an external proxy, use ListenTLS/ListenAutoTLS instead")
	}
}

// ListenTLS starts the HTTPS server with the given certificate and key.
// If certFile and keyFile are empty and tlsConfig is set,
// uses the TLS config directly.
func (a *App) ListenTLS(addr, certFile, keyFile string) error {
	if err := a.Err(); err != nil {
		return err
	}
	if err := a.fireOnListen(addr); err != nil {
		return err
	}
	srv := a.newServer()
	if a.tlsConfig != nil {
		srv.TLSConfig = a.tlsConfig
	}
	a.printStartupBanner(addr, true)
	return srv.ListenAndServeTLS(addr, certFile, keyFile)
}

// ListenAutoTLS starts the HTTPS server with automatic certificates
// via Let's Encrypt (ACME). The domain parameter selects which hosts
// the certificate will cover.
func (a *App) ListenAutoTLS(addr string, domain ...string) error {
	if len(domain) == 0 {
		return a.ListenTLS(addr, "", "")
	}
	if err := a.fireOnListen(addr); err != nil {
		return err
	}

	m := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
	}
	if len(domain) > 0 {
		m.HostPolicy = autocert.HostWhitelist(domain...)
	}

	srv := a.newServer()
	srv.TLSConfig = &tls.Config{
		GetCertificate: m.GetCertificate,
	}
	return srv.ListenAndServeTLS(addr, "", "")
}

// EnableTLS applies an explicit TLS config to the server.
func (a *App) EnableTLS(tlsConfig *tls.Config) *App {
	a.tlsConfig = tlsConfig
	return a
}

// Shutdown shuts the server down gracefully.
func (a *App) Shutdown() error {
	var err error
	if srv := a.currentServer(); srv != nil {
		err = srv.Shutdown()
	}
	a.fireOnShutdown()
	return err
}

// handleError processes a handler/middleware error using the
// custom ErrorHandler if configured, or the default fallback.
func (a *App) handleError(c *domain.Ctx, err error) {
	if err == errResponseWritten {
		return
	}
	if a.errorHandler != nil {
		if customErr := a.errorHandler(c, err); customErr != nil {
			usecases.HandleErrorEnv(c, customErr, a.env)
		}
		return
	}
	usecases.HandleErrorEnv(c, err, a.env)
}

// EnableMetrics turns on per-middleware metrics collection.
// Returns the collector for inspection.
func (a *App) EnableMetrics() *MetricsCollector {
	a.metrics = NewMetricsCollector()
	a.metrics.Enable()
	return a.metrics
}

// Metrics returns the metrics collector (nil if disabled).
func (a *App) Metrics() *MetricsCollector {
	return a.metrics
}

// GracefulShutdownConfig configures graceful shutdown behavior.
type GracefulShutdownConfig struct {
	// Timeout is the maximum time to wait for a complete shutdown.
	// Default: 30 seconds.
	Timeout time.Duration
	// Signals is the list of signals that trigger shutdown.
	// Default: SIGINT, SIGTERM.
	Signals []os.Signal
	// OnShutdown is called after the server stops (resource cleanup).
	OnShutdown func()
}

// ShutdownWithContext shuts the server down gracefully using the context
// for timeout control. If the context expires, shutdown is forced.
func (a *App) ShutdownWithContext(ctx context.Context) error {
	var err error
	if srv := a.currentServer(); srv != nil {
		done := make(chan error, 1)
		go func() {
			done <- srv.Shutdown()
		}()
		select {
		case err = <-done:
		case <-ctx.Done():
			err = ctx.Err()
		}
	}
	a.fireOnShutdown()
	return err
}

// ListenWithGracefulShutdown starts the server and waits for signals to
// shut down gracefully. Blocks until shutdown completes.
func (a *App) ListenWithGracefulShutdown(addr string, cfg ...GracefulShutdownConfig) error {
	return a.listenWithSignals(func() error { return a.Listen(addr) }, cfg...)
}

// ListenWithGracefulShutdownTLS is ListenTLS with signal-based graceful shutdown.
func (a *App) ListenWithGracefulShutdownTLS(addr, certFile, keyFile string, cfg ...GracefulShutdownConfig) error {
	return a.listenWithSignals(func() error { return a.ListenTLS(addr, certFile, keyFile) }, cfg...)
}

// ListenWithGracefulShutdownAutoTLS is ListenAutoTLS with signal-based graceful shutdown.
func (a *App) ListenWithGracefulShutdownAutoTLS(addr string, domains []string, cfg ...GracefulShutdownConfig) error {
	return a.listenWithSignals(func() error { return a.ListenAutoTLS(addr, domains...) }, cfg...)
}

func (a *App) listenWithSignals(start func() error, cfg ...GracefulShutdownConfig) error {
	config := GracefulShutdownConfig{
		Timeout: 30 * time.Second,
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	}
	if len(cfg) > 0 {
		if cfg[0].Timeout > 0 {
			config.Timeout = cfg[0].Timeout
		}
		if len(cfg[0].Signals) > 0 {
			config.Signals = cfg[0].Signals
		}
		if cfg[0].OnShutdown != nil {
			config.OnShutdown = cfg[0].OnShutdown
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, config.Signals...)

	errCh := make(chan error, 1)
	go func() {
		errCh <- start()
	}()

	select {
	case err := <-errCh:
		return err
	case <-sigCh:
		if !a.disableStartupMessage {
			fmt.Fprintln(os.Stdout, "  Nitro: signal received, shutting down…")
		}
		ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
		defer cancel()
		err := a.ShutdownWithContext(ctx)
		if config.OnShutdown != nil {
			config.OnShutdown()
		}
		return err
	}
}
