package nitro

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	nitroadapter "github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/ports/v2"
)

// TokenProvider generates and validates tokens (re-export of ports).
type TokenProvider = ports.TokenProvider

// UserLookup looks up users by email/ID.
type UserLookup = ports.UserLookup

// UserCreator persists new users.
type UserCreator = ports.UserCreator

// Auth is a fluent builder around AuthUseCase.
//
//	auth := nitro.NewAuth(cfg).WithMemoryStore()
//	app.AuthRoutes("/auth", auth)
type Auth struct {
	*AuthUseCase
	deps AuthDeps

	// Per-client-IP limit shared by AuthRoutes endpoints (credential
	// stuffing / brute force). Defaults: DefaultAuthRateLimit per minute.
	rateMax    int
	rateWindow time.Duration

	// Per-account limit of failed logins (see WithAccountRateLimit).
	accountMax    int
	accountWindow time.Duration
}

// Defaults of the per-account login limit: failed attempts allowed for one
// e-mail (or Keycloak username) inside the window, from any IP.
const (
	DefaultAuthAccountRateLimit  = 10
	DefaultAuthAccountRateWindow = 15 * time.Minute
)

// WithAccountRateLimit changes how many failed logins one account may
// accumulate inside window before /login answers 429 for it, whatever the
// client IP. The per-IP limit alone does not stop a password-guessing run
// spread over many addresses; this one does. A successful login clears the
// count. max <= 0 disables it.
func (a *Auth) WithAccountRateLimit(max int, window time.Duration) *Auth {
	a.accountMax, a.accountWindow = max, window
	return a
}

// loginThrottle counts failed logins per account.
type loginThrottle struct {
	limiter    domain.RateLimiter
	retryAfter string
}

func newLoginThrottle(max int, window time.Duration) *loginThrottle {
	if max <= 0 || window <= 0 {
		return nil
	}
	return &loginThrottle{
		limiter:    nitroadapter.NewSlidingWindowLimiter(max, window),
		retryAfter: strconv.Itoa(int((window + time.Second - 1) / time.Second)),
	}
}

// accountKey normalizes the login identifier; the length cap keeps a
// request from planting arbitrarily large keys in the limiter.
func accountKey(login string) string {
	login = strings.ToLower(strings.TrimSpace(login))
	if len(login) > 254 {
		login = login[:254]
	}
	return login
}

// blocked answers 429 when the account used up its failed attempts.
func (t *loginThrottle) blocked(c *Ctx, key string) error {
	if t == nil || t.limiter.Remaining(key) > 0 {
		return nil
	}
	c.SetHeader("Retry-After", t.retryAfter)
	return Err(429, "TOO_MANY_ATTEMPTS", "too many failed login attempts for this account; try again later")
}

func (t *loginThrottle) failed(key string) {
	if t != nil {
		t.limiter.Allow(key)
	}
}

func (t *loginThrottle) succeeded(key string) {
	if t != nil {
		t.limiter.Reset(key)
	}
}

// DefaultAuthRateLimit is how many AuthRoutes requests (register, login,
// refresh, logout combined) one client IP may make per minute by default.
const DefaultAuthRateLimit = 20

// WithRateLimit changes the per-IP limit of AuthRoutes; max <= 0 disables it
// (only when an upstream gateway already throttles these endpoints).
func (a *Auth) WithRateLimit(max int, window time.Duration) *Auth {
	a.rateMax, a.rateWindow = max, window
	return a
}

// NewAuth creates an Auth builder with JWT provider and bcrypt hasher.
// Call WithMemoryStore() or WithStore() before AuthRoutes.
func NewAuth(cfg AuthConfig) *Auth {
	a := &Auth{
		rateMax:       DefaultAuthRateLimit,
		rateWindow:    time.Minute,
		accountMax:    DefaultAuthAccountRateLimit,
		accountWindow: DefaultAuthAccountRateWindow,
		deps: AuthDeps{
			TokenProvider: JWTProvider(cfg),
			PasswordHash:  PasswordHasherAdapter(),
			SessionStore:  MemorySessionStore(),
		},
	}
	return a.rebuild()
}

// WithMemoryStore attaches an in-memory user store (lookup + create).
func (a *Auth) WithMemoryStore() *Auth {
	store := NewMemoryUserStore()
	a.deps.UserLookup = store
	a.deps.UserCreator = store
	return a.rebuild()
}

// WithStore sets custom user lookup/creator.
func (a *Auth) WithStore(lookup UserLookup, creator UserCreator) *Auth {
	a.deps.UserLookup = lookup
	a.deps.UserCreator = creator
	return a.rebuild()
}

// WithSessionStore sets the session store (default: memory).
func (a *Auth) WithSessionStore(store SessionStore) *Auth {
	a.deps.SessionStore = store
	return a.rebuild()
}

// WithPasswordHasher overrides the default bcrypt hasher.
func (a *Auth) WithPasswordHasher(h PasswordHasher) *Auth {
	a.deps.PasswordHash = h
	return a.rebuild()
}

// WithTokenProvider overrides the JWT provider from NewAuth.
func (a *Auth) WithTokenProvider(p TokenProvider) *Auth {
	a.deps.TokenProvider = p
	return a.rebuild()
}

// Provider returns the configured token provider (for JWTMiddleware).
func (a *Auth) Provider() TokenProvider {
	return a.deps.TokenProvider
}

func (a *Auth) rebuild() *Auth {
	a.AuthUseCase = NewAuthUseCase(a.deps)
	return a
}

// AuthRoutes registers POST {prefix}/register, /login, /refresh, /logout,
// rate-limited per client IP (see WithRateLimit); /login is also limited per
// account (see WithAccountRateLimit).
//
//	auth := nitro.NewAuth(cfg).WithMemoryStore()
//	app.AuthRoutes("/auth", auth)
func (a *App) AuthRoutes(prefix string, auth *Auth) *App {
	if auth == nil || auth.AuthUseCase == nil {
		return a.fail(errors.New("nitro: AuthRoutes requires auth from NewAuth(...).WithMemoryStore() (or WithStore)"))
	}
	if auth.deps.UserLookup == nil || auth.deps.UserCreator == nil {
		return a.fail(errors.New("nitro: AuthRoutes requires WithMemoryStore() or WithStore() before use"))
	}
	prefix = "/" + strings.Trim(prefix, "/")
	var mws []Handler
	if auth.rateMax > 0 && auth.rateWindow > 0 {
		mws = append(mws, RateLimit(auth.rateMax, auth.rateWindow))
	}
	g := a.Group(prefix, mws...)
	g.Post("/register", authRegisterHandler(auth.AuthUseCase))
	g.Post("/login", authLoginHandler(auth.AuthUseCase, newLoginThrottle(auth.accountMax, auth.accountWindow)))
	g.Post("/refresh", authRefreshHandler(auth.AuthUseCase))
	g.Post("/logout", authLogoutHandler(auth.AuthUseCase))
	a.warnMemoryRevocation(auth.deps.TokenProvider)
	return a
}

// KeycloakAuthRoutes registers POST {prefix}/login and /refresh (Keycloak
// password grant), rate-limited per client IP like AuthRoutes
// (DefaultAuthRateLimit per minute) and, on /login, per username
// (DefaultAuthAccountRateLimit failed attempts per 15 minutes) — the
// password grant is a credential stuffing target just the same. Pass extra
// middlewares (e.g. a stricter
// nitro.RateLimit) to add to it.
//
//	kc, _ := nitro.NewKeycloak(cfg)
//	app.KeycloakAuthRoutes("/auth", kc)
//	app.MeRoute(nitro.JWTMiddleware(kc.Provider()))
func (a *App) KeycloakAuthRoutes(prefix string, kc *KeycloakService, mws ...Handler) *App {
	if kc == nil {
		return a.fail(errors.New("nitro: KeycloakAuthRoutes requires *KeycloakService"))
	}
	prefix = "/" + strings.Trim(prefix, "/")
	mws = append([]Handler{RateLimit(DefaultAuthRateLimit, time.Minute)}, mws...)
	g := a.Group(prefix, mws...)
	tokens := kc.Tokens()
	g.Post("/login", keycloakLoginHandler(tokens, newLoginThrottle(DefaultAuthAccountRateLimit, DefaultAuthAccountRateWindow)))
	g.Post("/refresh", keycloakRefreshHandler(tokens))
	return a
}

func keycloakLoginHandler(tokens KeycloakClient, throttle *loginThrottle) Handler {
	return func(c *Ctx) error {
		body, err := Parse[struct {
			Username string `json:"username" validate:"required"`
			Password string `json:"password" validate:"required"`
		}](c)
		if err != nil {
			return err
		}
		key := accountKey(body.Username)
		if err := throttle.blocked(c, key); err != nil {
			return err
		}
		out, err := tokens.Login(c, body.Username, body.Password)
		if err != nil {
			// Only a rejected credential counts: an unreachable Keycloak
			// must not lock accounts out.
			var httpErr domain.HTTPError
			if errors.As(err, &httpErr) && (httpErr.StatusCode() == 401 || httpErr.StatusCode() == 400) {
				throttle.failed(key)
			}
			return err
		}
		throttle.succeeded(key)
		return c.OK(out)
	}
}

func keycloakRefreshHandler(tokens KeycloakClient) Handler {
	return func(c *Ctx) error {
		body, err := Parse[struct {
			RefreshToken string `json:"refresh_token" validate:"required"`
		}](c)
		if err != nil {
			return err
		}
		out, err := tokens.Refresh(c, body.RefreshToken)
		if err != nil {
			return err
		}
		return c.OK(out)
	}
}

func authRegisterHandler(auth *AuthUseCase) Handler {
	return func(c *Ctx) error {
		body, err := Parse[struct {
			Email string `json:"email" validate:"required,email"`
			// bcrypt only uses the first 72 bytes; longer input is rejected
			// instead of silently truncated (or failing as an opaque error).
			Password string `json:"password" validate:"required,min=8,max=72"`
			Name     string `json:"name" validate:"max=200"`
		}](c)
		if err != nil {
			return err
		}
		pair, err := auth.Register(c, body.Email, body.Password, body.Name)
		if errors.Is(err, domain.ErrEmailAlreadyExists) {
			return Err(409, "EMAIL_EXISTS", "email already exists")
		}
		if err != nil {
			return err // generic 500: never echo store/hash internals
		}
		return c.Created(Map{
			"access_token":  pair.AccessToken,
			"refresh_token": pair.RefreshToken,
			"token_type":    pair.TokenType,
			"expires_in":    pair.ExpiresIn,
		})
	}
}

// warnMemoryRevocation logs, in production, that logout and refresh-token
// rotation are only known to this process. It is a warning and not a
// configuration error because a single-replica deployment is fine with it.
func (a *App) warnMemoryRevocation(p TokenProvider) {
	if a.cfg.Env != EnvProduction {
		return
	}
	if m, ok := p.(interface{ UsesMemoryRevocation() bool }); ok && m.UsesMemoryRevocation() {
		slog.Warn("nitro: JWT revocations are kept in memory: with more than one replica, a logout or a rotated refresh token is not seen by the others. Use a shared store (redisrevocation) via SetRevocationStore.")
	}
}

func authLoginHandler(auth *AuthUseCase, throttle *loginThrottle) Handler {
	return func(c *Ctx) error {
		body, err := Parse[struct {
			Email    string `json:"email" validate:"required,email"`
			Password string `json:"password" validate:"required"`
		}](c)
		if err != nil {
			return err
		}
		key := accountKey(body.Email)
		if err := throttle.blocked(c, key); err != nil {
			return err
		}
		pair, err := auth.Login(c, body.Email, body.Password)
		if err != nil {
			throttle.failed(key)
			return Err(401, "INVALID_CREDENTIALS", "invalid credentials")
		}
		throttle.succeeded(key)
		return c.OK(Map{
			"access_token":  pair.AccessToken,
			"refresh_token": pair.RefreshToken,
			"token_type":    pair.TokenType,
			"expires_in":    pair.ExpiresIn,
		})
	}
}

// authLogoutHandler revokes the presented tokens: the refresh token from the
// body and the access token from the Authorization header. Always 204, so it
// reveals nothing about token validity.
func authLogoutHandler(auth *AuthUseCase) Handler {
	return func(c *Ctx) error {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = c.Bind(&body)
		access := ""
		if h := c.Header("Authorization"); len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
			access = strings.TrimSpace(h[7:])
		}
		auth.RevokeTokens(c, body.RefreshToken, access)
		return c.NoContent()
	}
}

func authRefreshHandler(auth *AuthUseCase) Handler {
	return func(c *Ctx) error {
		body, err := Parse[struct {
			RefreshToken string `json:"refresh_token" validate:"required"`
		}](c)
		if err != nil {
			return err
		}
		pair, err := auth.RefreshToken(c, body.RefreshToken)
		if err != nil {
			return Err(401, "INVALID_TOKEN", "invalid or expired refresh token")
		}
		return c.OK(Map{
			"access_token":  pair.AccessToken,
			"refresh_token": pair.RefreshToken,
			"token_type":    pair.TokenType,
			"expires_in":    pair.ExpiresIn,
		})
	}
}
