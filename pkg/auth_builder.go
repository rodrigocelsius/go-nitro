package nitro

import (
	"errors"
	"strings"
	"time"

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
		rateMax:    DefaultAuthRateLimit,
		rateWindow: time.Minute,
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
// rate-limited per client IP (see WithRateLimit).
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
	g.Post("/login", authLoginHandler(auth.AuthUseCase))
	g.Post("/refresh", authRefreshHandler(auth.AuthUseCase))
	g.Post("/logout", authLogoutHandler(auth.AuthUseCase))
	return a
}

// KeycloakAuthRoutes registers POST {prefix}/login and /refresh (Keycloak password grant).
//
//	kc, _ := nitro.NewKeycloak(cfg)
//	app.KeycloakAuthRoutes("/auth", kc)
//	app.MeRoute(nitro.JWTMiddleware(kc.Provider()))
func (a *App) KeycloakAuthRoutes(prefix string, kc *KeycloakService) *App {
	if kc == nil {
		return a.fail(errors.New("nitro: KeycloakAuthRoutes requires *KeycloakService"))
	}
	prefix = "/" + strings.Trim(prefix, "/")
	g := a.Group(prefix)
	tokens := kc.Tokens()
	g.Post("/login", keycloakLoginHandler(tokens))
	g.Post("/refresh", keycloakRefreshHandler(tokens))
	return a
}

func keycloakLoginHandler(tokens KeycloakClient) Handler {
	return func(c *Ctx) error {
		body, err := Parse[struct {
			Username string `json:"username" validate:"required"`
			Password string `json:"password" validate:"required"`
		}](c)
		if err != nil {
			return err
		}
		out, err := tokens.Login(c, body.Username, body.Password)
		if err != nil {
			return err
		}
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

func authLoginHandler(auth *AuthUseCase) Handler {
	return func(c *Ctx) error {
		body, err := Parse[struct {
			Email    string `json:"email" validate:"required,email"`
			Password string `json:"password" validate:"required"`
		}](c)
		if err != nil {
			return err
		}
		pair, err := auth.Login(c, body.Email, body.Password)
		if err != nil {
			return Err(401, "INVALID_CREDENTIALS", "invalid credentials")
		}
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
