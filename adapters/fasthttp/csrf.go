package fasthttp

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// CSRFConfig configures CSRF protection using the Double Submit Cookie pattern.
type CSRFConfig struct {
	CookieName   string   // Cookie name (default: "_csrf")
	HeaderName   string   // Header name (default: "X-CSRF-Token")
	CookiePath   string   // Cookie path (default: "/")
	CookieDomain string   // Cookie domain (optional)
	Secret       string   // Optional secret for HMAC-based tokens
	Methods      []string // Methods to protect (default: POST, PUT, DELETE, PATCH)
	IgnorePaths  []string // Paths to ignore (e.g., "/webhooks/*")
}

func csrfDefaults(cfg CSRFConfig) CSRFConfig {
	if cfg.CookieName == "" {
		cfg.CookieName = "_csrf"
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-CSRF-Token"
	}
	if cfg.CookiePath == "" {
		cfg.CookiePath = "/"
	}
	if len(cfg.Methods) == 0 {
		cfg.Methods = []string{"POST", "PUT", "DELETE", "PATCH"}
	}
	return cfg
}

// StandaloneCSRFToken returns a handler that generates and sets a CSRF token.
//
// Usage:
//
//	app.Get("/auth/csrf", nitro.CSRFToken())
//	app.Get("/auth/csrf", nitro.CSRFToken(nitro.CSRFConfig{CookieName: "xsrf"}))
func StandaloneCSRFToken(cfg ...CSRFConfig) domain.Handler {
	c := csrfDefaults(firstCSRFConfig(cfg...))
	return func(ctx *domain.Ctx) error {
		token, err := generateSecureCSRFToken()
		if err != nil {
			return domain.NewInternal(err)
		}
		setCSRFCookie(ctx, token, c)
		fc := getFastCtx(ctx)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}
		return fc.JSON(200, map[string]string{"csrf_token": token})
	}
}

// StandaloneCSRFMiddleware returns a middleware that validates CSRF tokens
// using the Double Submit Cookie pattern.
//
// Usage:
//
//	app.Use(nitro.CSRFMiddleware())
//	app.Use(nitro.CSRFMiddleware(nitro.CSRFConfig{Secret: "my-secret"}))
func StandaloneCSRFMiddleware(cfg ...CSRFConfig) domain.Handler {
	c := csrfDefaults(firstCSRFConfig(cfg...))
	methodSet := make(map[string]bool, len(c.Methods))
	for _, m := range c.Methods {
		methodSet[strings.ToUpper(m)] = true
	}
	ignoreSet := make(map[string]bool, len(c.IgnorePaths))
	for _, p := range c.IgnorePaths {
		ignoreSet[p] = true
	}

	return func(ctx *domain.Ctx) error {
		fc := getFastCtx(ctx)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		method := strings.ToUpper(fc.Method())
		if !methodSet[method] {
			return nil
		}

		path := fc.Path()
		for ignored := range ignoreSet {
			if pathMatchesIgnorePrefix(path, ignored) {
				return nil
			}
		}

		headerToken := fc.Header(c.HeaderName)
		if headerToken == "" {
			return domain.NewForbidden("missing CSRF token in header")
		}

		cookieToken := fc.Cookie(c.CookieName)
		if cookieToken == "" {
			return domain.NewForbidden("missing CSRF cookie")
		}

		if !csrfTokenEqual(headerToken, cookieToken) {
			return domain.NewForbidden("invalid CSRF token")
		}

		return nil
	}
}

// pathMatchesIgnorePrefix reports whether path is covered by an
// IgnorePaths entry. A plain strings.HasPrefix(path, ignored) let
// "/webhooks" exempt "/webhooks-admin/delete-all" from CSRF checks too —
// any sibling path merely sharing the prefix string, not the directory
// (CWE-863). Matching now requires either an exact path, or the ignored
// entry to be a proper path segment prefix (ignored+"/…"), plus a
// trailing "/*" glob as an explicit segment wildcard.
func pathMatchesIgnorePrefix(path, ignored string) bool {
	ignored = strings.TrimSuffix(ignored, "*")
	ignored = strings.TrimSuffix(ignored, "/")
	if ignored == "" {
		return true // IgnorePaths: ["/"] or ["*"] means "ignore everything"
	}
	if path == ignored {
		return true
	}
	return strings.HasPrefix(path, ignored+"/")
}

func setCSRFCookie(ctx *domain.Ctx, token string, cfg CSRFConfig) {
	fc := getFastCtx(ctx)
	if fc == nil {
		return
	}
	secure := strings.EqualFold(string(fc.FasthttpCtx().URI().Scheme()), "https")
	fc.SetCookieWithSameSite(cfg.CookieName, token, 3600, cfg.CookiePath, cfg.CookieDomain, secure, false, fasthttp.CookieSameSiteStrictMode)
}

func generateSecureCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func csrfTokenEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func firstCSRFConfig(cfg ...CSRFConfig) CSRFConfig {
	if len(cfg) > 0 {
		return cfg[0]
	}
	return CSRFConfig{}
}
