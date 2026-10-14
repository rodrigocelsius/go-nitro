package fasthttp

import (
	"errors"
	"net/http"
	"net/http/pprof"
	"net/url"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// PprofHandler returns a domain.Handler that serves pprof profiles.
// Register on individual /debug/pprof/* routes to enable profiling.
//
// Example:
//
//	fasthttp.SetupPprof(app)
func PprofHandler() domain.Handler {
	return func(c *domain.Ctx) error {
		fc, ok := c.AdapterData.(*FastCtx)
		if !ok || fc == nil {
			return nil
		}
		fctx := fc.FasthttpCtx()
		path := string(fctx.Path())

		profile := strings.TrimPrefix(path, "/debug/pprof")
		profile = strings.TrimPrefix(profile, "/")

		rw := &responseWriter{ctx: fctx}
		req := buildNetHTTPRequest(fctx)

		if profile == "profile" || profile == "trace" {
			if secs := fctx.QueryArgs().GetUintOrZero("seconds"); secs > pprofMaxSeconds {
				return domain.NewBadRequest("pprof: seconds must be <= 30")
			}
			select {
			case pprofSlots <- struct{}{}:
				defer func() { <-pprofSlots }()
			default:
				return &domain.TooManyRequestsError{}
			}
		}

		switch profile {
		case "", "index":
			pprof.Index(rw, req)
		case "cmdline":
			pprof.Cmdline(rw, req)
		case "profile":
			pprof.Profile(rw, req)
		case "symbol":
			pprof.Symbol(rw, req)
		case "trace":
			pprof.Trace(rw, req)
		default:
			pprof.Index(rw, req)
		}

		fctx.Response.SetBody(rw.body)
		if rw.statusCode > 0 {
			fctx.SetStatusCode(rw.statusCode)
		}
		if ct := rw.headers.Get("Content-Type"); ct != "" {
			fctx.Response.Header.SetContentType(ct)
		}
		return nil
	}
}

// pprofMaxSeconds caps ?seconds= on /profile and /trace, and pprofSlots
// allows one long capture at a time: each capture holds a goroutine and
// buffers the whole result in memory, so unbounded concurrent captures are a
// cheap DoS.
const pprofMaxSeconds = 30

var pprofSlots = make(chan struct{}, 1)

// SetupPprof registers all pprof routes on the app.
//
// pprof exposes memory contents, goroutine stacks and on-demand profiling, so
// in production (Env == EnvProduction) at least one protecting middleware is
// REQUIRED — SetupPprof panics at registration otherwise (CWE-200/CWE-306;
// vuln.md V-08):
//
//	fasthttp.SetupPprof(app, fasthttp.IPWhitelistMiddleware("10.0.0.0/8"))
//
// Outside production the middlewares are optional.
func SetupPprof(app *App, mws ...domain.Handler) {
	if app.env == domain.EnvProduction && len(mws) == 0 {
		app.AddConfigError(errors.New("nitro: SetupPprof in production requires an auth/network middleware, e.g. SetupPprof(app, IPWhitelistMiddleware(\"10.0.0.0/8\")) — refusing to expose /debug/pprof/* unauthenticated"))
		return
	}
	handler := PprofHandler()
	for _, p := range []string{"", "index", "cmdline", "profile", "symbol", "trace",
		"heap", "goroutine", "block", "mutex", "threadcreate", "allocs"} {
		path := "/debug/pprof/" + p
		if p == "" {
			path = "/debug/pprof/"
		}
		app.Get(path, handler, mws...)
	}
}

// responseWriter adapts fasthttp to net/http.ResponseWriter.
type responseWriter struct {
	ctx        *fasthttp.RequestCtx
	headers    http.Header
	body       []byte
	statusCode int
}

func (w *responseWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// buildNetHTTPRequest builds an *http.Request from fasthttp.RequestCtx
// to satisfy the interface expected by net/http/pprof.
func buildNetHTTPRequest(fctx *fasthttp.RequestCtx) *http.Request {
	uri := fctx.Request.URI()
	return &http.Request{
		Method: string(fctx.Method()),
		URL: &url.URL{
			Scheme:   string(uri.Scheme()),
			Host:     string(uri.Host()),
			Path:     string(uri.Path()),
			RawQuery: string(uri.QueryString()),
		},
		Header:        make(http.Header),
		ContentLength: int64(fctx.Request.Header.ContentLength()),
		Host:          string(fctx.Host()),
		RemoteAddr:    fctx.RemoteAddr().String(),
		RequestURI:    string(fctx.RequestURI()),
	}
}
