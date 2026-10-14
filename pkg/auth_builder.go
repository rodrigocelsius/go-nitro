package nitro

import (
	"errors"
	"strings"

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
}

// NewAuth creates an Auth builder with JWT provider and bcrypt hasher.
// Call WithMemoryStore() or WithStore() before AuthRoutes.
func NewAuth(cfg AuthConfig) *Auth {
	a := &Auth{
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

// AuthRoutes registers POST {prefix}/register, /login, /refresh.
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
	g := a.Group(prefix)
	g.Post("/register", authRegisterHandler(auth.AuthUseCase))
	g.Post("/login", authLoginHandler(auth.AuthUseCase))
	g.Post("/refresh", authRefreshHandler(auth.AuthUseCase))
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
			Email    string `json:"email" validate:"required,email"`
			Password string `json:"password" validate:"required,min=8"`
			Name     string `json:"name"`
		}](c)
		if err != nil {
			return err
		}
		pair, err := auth.Register(c, body.Email, body.Password, body.Name)
		if err != nil {
			return Err(409, "EMAIL_EXISTS", err.Error())
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
