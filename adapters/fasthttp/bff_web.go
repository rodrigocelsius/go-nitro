package fasthttp

import (
	"strings"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/ports/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
	"github.com/valyala/fasthttp"
)

// WebBFFConfig configures the BFF for web applications.
type WebBFFConfig struct {
	BFFConfig // embeds base config

	// Token storage (cookies)
	TokenCookieName   string `json:"token_cookie_name"`   // "_at"
	RefreshCookieName string `json:"refresh_cookie_name"` // "_rt"
	CSRFCookieName    string `json:"csrf_cookie_name"`    // "_csrf"
	CookieDomain      string `json:"cookie_domain"`
	CookiePath        string `json:"cookie_path"`      // "/"
	CookieSecure      bool   `json:"cookie_secure"`    // true in production
	CookieSameSite    string `json:"cookie_same_site"` // "Lax"
	CookieHTTPOnly    bool   `json:"cookie_http_only"` // always true

	// Token TTL
	AccessTokenTTL  time.Duration `json:"access_token_ttl"`  // 15min
	RefreshTokenTTL time.Duration `json:"refresh_token_ttl"` // 7d

	// Session (server-side)
	SessionStore domain.SessionStore `json:"-"`
	SessionTTL   time.Duration       `json:"session_ttl"` // 7d

	// Security
	EnableCSRF     bool     `json:"enable_csrf"` // true by default
	TrustedOrigins []string `json:"trusted_origins"`

	// Web-specific
	EnableSSR  bool          `json:"enable_ssr"`  // server-side rendering
	SSRTimeout time.Duration `json:"ssr_timeout"` // 10s
}

// === Web BFF Middleware ===

// WebBFFMiddleware returns a middleware that:
// 1. Reads access token from HttpOnly cookie
// 2. Validates the token
// 3. If expired, attempts refresh via refresh token cookie
// 4. Injects headers for downstream services
// 5. Adds BFF identification headers
//
// Usage:
//
//	web := app.Group("/web", nitro.WebBFFMiddleware(webConfig))
//	web.Get("/users", nitro.BFFProxyHandler(webConfig, "users", "/api/users"))
func WebBFFMiddleware(cfg WebBFFConfig, tokenProvider ports.TokenProvider) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		// 1. Read access token from cookie
		accessToken := fc.Cookie(cfg.TokenCookieName)
		if accessToken == "" {
			return domain.NewUnauthorized("no access token")
		}

		// 2. Validate token
		claims, err := tokenProvider.ValidateToken(c.Context(), accessToken)
		if err != nil {
			// Token expired → try refresh
			refreshToken := fc.Cookie(cfg.RefreshCookieName)
			if refreshToken == "" {
				return domain.NewUnauthorized("session expired")
			}

			newTokens, err := tokenProvider.RefreshToken(c.Context(), refreshToken)
			if err != nil {
				clearWebBFFCookies(fc, cfg)
				return domain.NewUnauthorized("session expired")
			}

			setWebBFFCookies(fc, newTokens, cfg)

			// Re-validate the new access token
			claims, err = tokenProvider.ValidateToken(c.Context(), newTokens.AccessToken)
			if err != nil {
				return domain.NewUnauthorized("token validation failed")
			}
			accessToken = newTokens.AccessToken
		}

		// 3. Inject data into context
		fc.Set("user_id", claims.UserID)
		fc.Set("email", claims.Email)
		fc.Set("roles", claims.Roles)
		fc.Set("claims", claims)
		if claims.TenantID != "" {
			fc.Set("tenant_id", claims.TenantID)
		}

		// 4. Inject headers on the REQUEST for downstream proxy (never on response)
		fc.SetRequestHeader("Authorization", "Bearer "+accessToken)
		fc.SetRequestHeader("X-Forwarded-By", "nitro-web-bff")
		fc.SetRequestHeader("X-User-ID", claims.UserID)
		fc.SetRequestHeader("X-Client-Type", "web")

		return nil
	}
}

// === Web BFF Login Handler ===

func WebBFFLoginHandler(auth *usecases.AuthUseCase, cfg WebBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := fc.BindJSON(&req); err != nil {
			return domain.NewBadRequest("invalid request")
		}

		tokens, err := auth.Login(c.Context(), req.Email, req.Password)
		if err != nil {
			return err
		}

		// Store in HttpOnly cookies (XSS-safe)
		setWebBFFCookies(fc, tokens, cfg)

		// Response WITHOUT tokens (security)
		return fc.JSON(200, map[string]string{"message": "login successful"})
	}
}

// === Web BFF Register Handler ===

func WebBFFRegisterHandler(auth *usecases.AuthUseCase, cfg WebBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Name     string `json:"name"`
		}

		if err := fc.BindJSON(&req); err != nil {
			return domain.NewBadRequest("invalid request")
		}

		tokens, err := auth.Register(c.Context(), req.Email, req.Password, req.Name)
		if err != nil {
			return err
		}

		setWebBFFCookies(fc, tokens, cfg)

		return fc.JSON(201, map[string]string{"message": "registration successful"})
	}
}

// === Web BFF Refresh Handler ===

func WebBFFRefreshHandler(auth *usecases.AuthUseCase, cfg WebBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		refreshToken := fc.Cookie(cfg.RefreshCookieName)
		if refreshToken == "" {
			return domain.NewUnauthorized("no refresh token")
		}

		tokens, err := auth.RefreshToken(c.Context(), refreshToken)
		if err != nil {
			clearWebBFFCookies(fc, cfg)
			return domain.NewUnauthorized("session expired")
		}

		setWebBFFCookies(fc, tokens, cfg)

		return fc.JSON(200, map[string]string{"message": "token refreshed"})
	}
}

// === Web BFF Logout Handler ===

func WebBFFLogoutHandler(auth *usecases.AuthUseCase, cfg WebBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		refreshToken := fc.Cookie(cfg.RefreshCookieName)
		if refreshToken != "" {
			// Attempt to revoke - ignore errors
			_ = auth.Logout(c.Context(), refreshToken)
		}

		clearWebBFFCookies(fc, cfg)

		return fc.JSON(200, map[string]string{"message": "logged out"})
	}
}

// === Helper Functions ===

func setWebBFFCookies(fc *FastCtx, tokens *domain.TokenPair, cfg WebBFFConfig) {
	setSecureCookie(fc, cfg.TokenCookieName, tokens.AccessToken,
		int(cfg.AccessTokenTTL.Seconds()), cfg)
	setSecureCookie(fc, cfg.RefreshCookieName, tokens.RefreshToken,
		int(cfg.RefreshTokenTTL.Seconds()), cfg)
}

func setSecureCookie(fc *FastCtx, name, value string, maxAge int, cfg WebBFFConfig) {
	path := cfg.CookiePath
	if path == "" {
		path = "/"
	}
	sameSite := parseSameSite(cfg.CookieSameSite)
	fc.SetCookieWithSameSite(name, value, maxAge, path, cfg.CookieDomain, cfg.CookieSecure, true, sameSite)
}

func parseSameSite(s string) fasthttp.CookieSameSite {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "strict":
		return fasthttp.CookieSameSiteStrictMode
	case "none":
		return fasthttp.CookieSameSiteNoneMode
	case "lax", "":
		return fasthttp.CookieSameSiteLaxMode
	default:
		return fasthttp.CookieSameSiteLaxMode
	}
}

func clearWebBFFCookies(fc *FastCtx, cfg WebBFFConfig) {
	setSecureCookie(fc, cfg.TokenCookieName, "", -1, cfg)
	setSecureCookie(fc, cfg.RefreshCookieName, "", -1, cfg)
}

// === CSRF Protection ===

// CSRFTokenHandler returns a handler that generates a CSRF token.
func CSRFTokenHandler(cfg WebBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		token, err := generateSecureCSRFToken()
		if err != nil {
			return domain.NewInternal(err)
		}

		path := cfg.CookiePath
		if path == "" {
			path = "/"
		}
		// CSRF cookie must be readable by JS (not HttpOnly) for double-submit.
		fc.SetCookieWithSameSite(cfg.CSRFCookieName, token, 3600, path, cfg.CookieDomain, cfg.CookieSecure, false, fasthttp.CookieSameSiteStrictMode)

		return fc.JSON(200, map[string]string{"csrf_token": token})
	}
}

// CSRFMiddleware returns a middleware that validates CSRF tokens for state-changing methods.
func CSRFMiddleware(cfg WebBFFConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		fc := getFastCtx(c)
		if fc == nil {
			return domain.NewInternal(domain.ErrNotFound)
		}

		method := fc.Method()
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return nil
		}

		if len(cfg.TrustedOrigins) > 0 {
			origin := fc.Header("Origin")
			if origin == "" {
				origin = fc.Header("Referer")
			}
			if !originTrusted(origin, cfg.TrustedOrigins) {
				return domain.NewForbidden("untrusted origin")
			}
		}

		headerToken := fc.Header("X-CSRF-Token")
		if headerToken == "" {
			return domain.NewForbidden("missing CSRF token")
		}

		cookieToken := fc.Cookie(cfg.CSRFCookieName)
		if cookieToken == "" {
			return domain.NewForbidden("missing CSRF cookie")
		}

		if !csrfTokenEqual(headerToken, cookieToken) {
			return domain.NewForbidden("invalid CSRF token")
		}

		return nil
	}
}

// originTrusted checks the request Origin/Referer against the configured
// allowlist. Matching requires an exact origin (scheme+host+port) or an
// exact host match against a leading "*." wildcard entry — never a prefix
// match. A naive strings.HasPrefix(origin, t) let an attacker-registered
// domain like "https://app.example.com.evil.com" pass a trusted entry of
// "https://app.example.com" (CWE-346), defeating CSRF protection entirely.
func originTrusted(origin string, trusted []string) bool {
	if origin == "" {
		return false
	}
	host := originHost(origin)
	for _, t := range trusted {
		if t == "*" || origin == t {
			return true
		}
		if wildcardSuffix, ok := strings.CutPrefix(t, "*."); ok {
			if host != "" && (host == wildcardSuffix || strings.HasSuffix(host, "."+wildcardSuffix)) {
				return true
			}
		}
	}
	return false
}

// originHost extracts the host (without scheme/port) from an Origin or
// Referer header value, e.g. "https://app.example.com:8443/x" → "app.example.com".
func originHost(origin string) string {
	rest := origin
	if idx := strings.Index(rest, "://"); idx >= 0 {
		rest = rest[idx+3:]
	}
	if idx := strings.IndexAny(rest, "/"); idx >= 0 {
		rest = rest[:idx]
	}
	if idx := strings.LastIndex(rest, ":"); idx >= 0 {
		rest = rest[:idx]
	}
	return rest
}

// === Route Registration Helpers ===

// RegisterWebBFFRoutes registers standard web BFF routes.
func RegisterWebBFFRoutes(app *App, prefix string, auth *usecases.AuthUseCase, cfg WebBFFConfig, tokenProvider ports.TokenProvider) {
	// Auth routes (public)
	app.Post(prefix+"/auth/login", WebBFFLoginHandler(auth, cfg))
	app.Post(prefix+"/auth/register", WebBFFRegisterHandler(auth, cfg))
	app.Post(prefix+"/auth/refresh", WebBFFRefreshHandler(auth, cfg))
	app.Post(prefix+"/auth/logout", WebBFFLogoutHandler(auth, cfg))

	// CSRF token endpoint
	if cfg.EnableCSRF {
		app.Get(prefix+"/auth/csrf", CSRFTokenHandler(cfg))
	}

	// Protected routes with BFF middleware
	protected := app.Group(prefix)
	protected.Use(WebBFFMiddleware(cfg, tokenProvider))
	if cfg.EnableCSRF {
		protected.Use(CSRFMiddleware(cfg))
	}

	// Example: proxy routes would be registered here
	// protected.Get("/users", BFFProxyHandler(cfg.BFFConfig, "users", "/api/users"))
}
