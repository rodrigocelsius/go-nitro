package fasthttp

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// Default CORS values
var (
	corsHeaderOrigin           = []byte("Origin")
	corsHeaderVary             = []byte("Vary")
	corsHeaderAllowOrigin      = []byte("Access-Control-Allow-Origin")
	corsHeaderAllowMethods     = []byte("Access-Control-Allow-Methods")
	corsHeaderAllowHeaders     = []byte("Access-Control-Allow-Headers")
	corsHeaderAllowCredentials = []byte("Access-Control-Allow-Credentials")
	corsHeaderMaxAge           = []byte("Access-Control-Max-Age")
	corsHeaderRequestMethod    = []byte("Access-Control-Request-Method")
	corsHeaderRequestHeaders   = []byte("Access-Control-Request-Headers")

	corsValueVaryOrigin = []byte("Origin")
	corsValueTrue       = []byte("true")
	corsValueWildcard   = []byte("*")
)

// CORSMiddleware returns a middleware that handles Cross-Origin Resource Sharing.
// Empty AllowedOrigins allows every origin ("*"): this constructor does not
// know the environment. Use CORSMiddlewareWithEnv (what app.CORS() does) for
// the convention "permissive in development, explicit origins in production".
func CORSMiddleware(cfg ...domain.CORSConfig) domain.Handler {
	var config domain.CORSConfig
	if len(cfg) > 0 {
		config = cfg[0]
	}
	if len(config.AllowedOrigins) == 0 {
		config.AllowedOrigins = []string{"*"}
	}
	// Security: reject credentials + wildcard combination
	if config.AllowCredentials && hasWildcard(config.AllowedOrigins) {
		config.AllowCredentials = false
	}
	return corsMiddleware(config)
}

// CORSMiddlewareWithEnv returns a CORS middleware that respects the environment.
// In development, it allows all origins by default. Outside development, empty
// AllowedOrigins allows no cross-origin request at all.
func CORSMiddlewareWithEnv(config domain.CORSConfig, env domain.Env) domain.Handler {
	if env == domain.EnvDevelopment && config.AllowedOrigins == nil && config.AllowedMethods == nil {
		config.AllowedOrigins = []string{"*"}
		config.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
		config.AllowedHeaders = []string{"*"}
	}
	// Security: reject credentials + wildcard combination
	if config.AllowCredentials && hasWildcard(config.AllowedOrigins) {
		config.AllowCredentials = false
	}
	return corsMiddleware(config)
}

func corsMiddleware(cfg domain.CORSConfig) domain.Handler {
	allowAll := hasWildcard(cfg.AllowedOrigins)

	allowMethods := []byte("GET,POST,PUT,DELETE,PATCH,HEAD,OPTIONS")
	if len(cfg.AllowedMethods) > 0 {
		allowMethods = []byte(strings.Join(cfg.AllowedMethods, ","))
	}

	allowHeaders := "Content-Type,Authorization"
	if len(cfg.AllowedHeaders) > 0 {
		allowHeaders = strings.Join(cfg.AllowedHeaders, ",")
	}
	reflectHeaders := allowHeaders == "*"
	allowHeadersB := []byte(allowHeaders)

	maxAge := []byte("600")
	if cfg.MaxAge > 0 {
		maxAge = []byte(strconv.Itoa(cfg.MaxAge))
	}

	// Allowlist: exact match on the serialized origin (scheme://host[:port]).
	// The lookup itself is the validation — only configured values are ever
	// echoed back — so origins with a port or a single-label host
	// (http://localhost:3000) work.
	originIndex := make(map[string]struct{}, len(cfg.AllowedOrigins))
	if !allowAll {
		for _, o := range cfg.AllowedOrigins {
			if o = normalizeOrigin(o); o != "" {
				originIndex[o] = struct{}{}
			}
		}
	}

	return func(c *domain.Ctx) error {
		fctx := c.AdapterData.(*FastCtx).FasthttpCtx()
		h := &fctx.Response.Header

		origin := fctx.Request.Header.PeekBytes(corsHeaderOrigin)
		if len(origin) == 0 {
			return nil
		}
		appendVary(h, corsValueVaryOrigin)

		allowed := allowAll
		if !allowed {
			_, allowed = originIndex[normalizeOrigin(b2s(origin))]
		}

		// Preflight request
		if methodBytes(fctx.Method(), "OPTIONS") && len(fctx.Request.Header.PeekBytes(corsHeaderRequestMethod)) > 0 {
			fctx.SetStatusCode(fasthttp.StatusNoContent)
			if !allowed {
				return nil // no CORS headers: the browser blocks the request
			}
			setAllowOrigin(h, origin, allowAll)
			h.SetCanonical(corsHeaderAllowMethods, allowMethods)
			// Allow headers (reflect request headers if needed)
			if reqHeaders := fctx.Request.Header.PeekBytes(corsHeaderRequestHeaders); reflectHeaders && len(reqHeaders) > 0 {
				h.SetCanonical(corsHeaderAllowHeaders, reqHeaders)
			} else {
				h.SetCanonical(corsHeaderAllowHeaders, allowHeadersB)
			}
			if cfg.AllowCredentials {
				h.SetCanonical(corsHeaderAllowCredentials, corsValueTrue)
			}
			h.SetCanonical(corsHeaderMaxAge, maxAge)
			return nil
		}

		// Actual request
		if !allowed {
			return nil
		}
		setAllowOrigin(h, origin, allowAll)
		if cfg.AllowCredentials {
			h.SetCanonical(corsHeaderAllowCredentials, corsValueTrue)
		}
		return nil
	}
}

func setAllowOrigin(h *fasthttp.ResponseHeader, origin []byte, allowAll bool) {
	if allowAll {
		h.SetCanonical(corsHeaderAllowOrigin, corsValueWildcard)
		return
	}
	h.SetCanonical(corsHeaderAllowOrigin, origin)
}

// normalizeOrigin lowercases an origin and drops a trailing slash, so a
// configured "https://App.example.com/" matches the browser's
// "https://app.example.com". Returns "" for "null" and values without a
// scheme. Allocation-free when the origin is already normalized.
func normalizeOrigin(o string) string {
	o = strings.TrimSuffix(strings.TrimSpace(o), "/")
	if o == "" || o == "null" || !strings.Contains(o, "://") {
		return ""
	}
	for i := 0; i < len(o); i++ {
		if c := o[i]; c >= 'A' && c <= 'Z' {
			return strings.ToLower(o)
		}
	}
	return o
}

// appendVary adds token to the Vary header, keeping what is already there.
// CORS and Compress both vary the response; overwriting dropped
// "Vary: Origin", so a shared cache could serve one origin's
// Access-Control-Allow-Origin to another.
func appendVary(h *fasthttp.ResponseHeader, token []byte) {
	cur := h.PeekBytes(corsHeaderVary)
	if len(cur) == 0 {
		h.SetCanonical(corsHeaderVary, token)
		return
	}
	for _, part := range bytes.Split(cur, []byte(",")) {
		if p := bytes.TrimSpace(part); bytes.EqualFold(p, token) || (len(p) == 1 && p[0] == '*') {
			return
		}
	}
	v := make([]byte, 0, len(cur)+2+len(token))
	v = append(append(append(v, cur...), ", "...), token...)
	h.SetCanonical(corsHeaderVary, v)
}

func hasWildcard(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return true
		}
	}
	return false
}
