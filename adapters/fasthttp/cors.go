package fasthttp

import (
	"net"
	"net/url"
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
)

// CORSMiddleware returns a middleware that handles Cross-Origin Resource Sharing.
// With no config, it allows all origins in development and requires explicit
// config in production.
func CORSMiddleware(cfg ...domain.CORSConfig) domain.Handler {
	var config domain.CORSConfig
	if len(cfg) > 0 {
		config = cfg[0]
	}
	// Security: reject credentials + wildcard combination
	if config.AllowCredentials && hasWildcard(config.AllowedOrigins) {
		config.AllowCredentials = false
	}
	return corsMiddleware(config)
}

// CORSMiddlewareWithEnv returns a CORS middleware that respects the environment.
// In development, it allows all origins by default. In production, it requires
// explicit configuration.
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
	allowOrigin := "*"
	if len(cfg.AllowedOrigins) > 0 && !hasWildcard(cfg.AllowedOrigins) {
		allowOrigin = ""
	}

	allowMethods := "GET,POST,PUT,DELETE,PATCH,HEAD,OPTIONS"
	if len(cfg.AllowedMethods) > 0 {
		allowMethods = strings.Join(cfg.AllowedMethods, ",")
	}

	allowHeaders := "Content-Type,Authorization"
	if len(cfg.AllowedHeaders) > 0 {
		allowHeaders = strings.Join(cfg.AllowedHeaders, ",")
	}

	maxAge := "600"
	if cfg.MaxAge > 0 {
		maxAge = strconv.Itoa(cfg.MaxAge)
	}

	originIndex := make(map[string]bool)
	if !hasWildcard(cfg.AllowedOrigins) {
		for _, o := range cfg.AllowedOrigins {
			originIndex[o] = true
		}
	}
	allowAll := hasWildcard(cfg.AllowedOrigins)

	return func(c *domain.Ctx) error {
		fctx := c.AdapterData.(*FastCtx).FasthttpCtx()
		h := &fctx.Response.Header

		origin := b2s(fctx.Request.Header.PeekBytes(corsHeaderOrigin))
		if origin == "" {
			return nil
		}

		// Preflight request
		if methodBytes(fctx.Method(), "OPTIONS") {
			reqMethod := b2s(fctx.Request.Header.PeekBytes(corsHeaderRequestMethod))
			if reqMethod != "" {
				// Set Vary header
				h.SetCanonical(corsHeaderVary, corsValueVaryOrigin)

				// Allow origin
				h.SetCanonical(corsHeaderAllowOrigin, []byte(resolveOrigin(origin, allowOrigin, allowAll, originIndex)))

				// Allow methods
				h.SetCanonical(corsHeaderAllowMethods, []byte(allowMethods))

				// Allow headers (reflect request headers if needed)
				reqHeaders := b2s(fctx.Request.Header.PeekBytes(corsHeaderRequestHeaders))
				effectiveHeaders := allowHeaders
				if reqHeaders != "" && allowHeaders == "*" {
					effectiveHeaders = reqHeaders
				}
				h.SetCanonical(corsHeaderAllowHeaders, []byte(effectiveHeaders))

				// Credentials
				if cfg.AllowCredentials {
					h.SetCanonical(corsHeaderAllowCredentials, []byte("true"))
				}

				// Max age
				h.SetCanonical(corsHeaderMaxAge, []byte(maxAge))

				fctx.SetStatusCode(fasthttp.StatusNoContent)
				return nil
			}
		}

		// Actual request
		h.SetCanonical(corsHeaderVary, corsValueVaryOrigin)
		h.SetCanonical(corsHeaderAllowOrigin, []byte(resolveOrigin(origin, allowOrigin, allowAll, originIndex)))

		if cfg.AllowCredentials {
			h.SetCanonical(corsHeaderAllowCredentials, []byte("true"))
		}

		return nil
	}
}

func resolveOrigin(origin, defaultOrigin string, allowAll bool, originIndex map[string]bool) string {
	if allowAll {
		return "*"
	}
	// Validate origin format before reflecting it
	if !isValidOrigin(origin) {
		return ""
	}
	if defaultOrigin == "" {
		if originIndex[origin] {
			return origin
		}
		return ""
	}
	return defaultOrigin
}

// isValidOrigin validates that an origin has a proper format
// to prevent CORS-based attacks via malformed origins.
func isValidOrigin(origin string) bool {
	if origin == "" || origin == "null" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	// Must have a scheme (http or https)
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	// Must have a valid host
	if u.Host == "" {
		return false
	}
	// Host must not contain special characters that could indicate injection
	if strings.ContainsAny(u.Host, "!@#$%^&*()+=[]{}|\\;:'\",<>?/") {
		return false
	}
	// Validate host is a valid IP or hostname
	host := u.Hostname()
	if net.ParseIP(host) != nil {
		return true // Valid IP address
	}
	// Validate as hostname (RFC 952 / RFC 1123)
	if !isValidHostname(host) {
		return false
	}
	return true
}

// isValidHostname validates that a string is a valid hostname.
func isValidHostname(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	// Check each label
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return false // Must have at least two labels (e.g., example.com)
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}
		// Must start and end with alphanumeric
		if !isAlphanumeric(label[0]) || !isAlphanumeric(label[len(label)-1]) {
			return false
		}
		// Can only contain alphanumeric and hyphens
		for i := 0; i < len(label); i++ {
			if !isAlphanumeric(label[i]) && label[i] != '-' {
				return false
			}
		}
	}
	return true
}

func isAlphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func hasWildcard(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return true
		}
	}
	return false
}
