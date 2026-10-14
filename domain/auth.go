package domain

import (
	"context"
	"time"
)

// === Interfaces (for dependency inversion) ===

// SessionStore manages server-side sessions.
type SessionStore interface {
	Create(ctx context.Context, session *Session) error
	Get(ctx context.Context, id string) (*Session, error)
	Update(ctx context.Context, session *Session) error
	Delete(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, userID string) error
}

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) bool
}

// === User ===

// User represents an authenticatable user.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // never serialized to JSON
	Name         string    `json:"name"`
	Roles        []string  `json:"roles"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// TenantID is the shared-schema tenant for this user when the app is
	// multi-tenant. Empty for single-tenant apps. When set, JWT access and
	// refresh tokens include a "tenant_id" claim so TenantMiddleware /
	// c.TenantID() can resolve without an extra header.
	TenantID string `json:"tenant_id,omitempty"`

	// EmailVerified reports whether the identity provider (OAuth2/OIDC)
	// confirmed ownership of Email. Only meaningful on the transient User
	// value returned by an oauth2.UserFunc — never persisted, and ignored
	// for password-based accounts (always verified there, since the user
	// authenticated with a secret only they know). LoginOAuth refuses to
	// link/authenticate into an existing account when this is false, to
	// prevent account takeover via a provider that allows unverified
	// emails at signup. See docs/guide/authentication.md#oauth2.
	EmailVerified bool `json:"-"`
}

// HasRole reports whether the user has the given role.
func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole reports whether the user has at least one of the roles.
func (u *User) HasAnyRole(roles ...string) bool {
	for _, required := range roles {
		if u.HasRole(required) {
			return true
		}
	}
	return false
}

// === Token ===

// TokenPair is the access+refresh pair returned on login.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
	TokenType    string `json:"token_type"` // "Bearer"
}

// === Claims ===

// Claims represents JWT claims.
type Claims struct {
	UserID string   `json:"user_id"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles"`
	// TenantID is the shared-schema tenant from the "tenant_id" claim when
	// present. Empty when the token was issued without a tenant (single-tenant
	// apps or older tokens). See TenantMiddleware / c.TenantID().
	TenantID string `json:"tenant_id,omitempty"`
	// Standard JWT fields (exp, iat, jti) are managed by TokenProvider
}

// HasRole reports whether the claims contain the given role.
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole reports whether the claims contain at least one of the roles.
func (c *Claims) HasAnyRole(roles ...string) bool {
	for _, required := range roles {
		if c.HasRole(required) {
			return true
		}
	}
	return false
}

// === Session ===

// Session represents a server-side session.
type Session struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Data      map[string]any `json:"data"`
	ExpiresAt time.Time      `json:"expires_at"`
	CreatedAt time.Time      `json:"created_at"`
}

// IsExpired reports whether the session has expired.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// === Device (Mobile) ===

// DeviceInfo represents a registered mobile device.
type DeviceInfo struct {
	DeviceID   string    `json:"device_id"`
	UserID     string    `json:"user_id"`
	Platform   string    `json:"platform"`    // "ios" | "android"
	FCMToken   string    `json:"fcm_token"`   // Firebase Cloud Messaging
	APNSToken  string    `json:"apns_token"`  // Apple Push Notification
	Model      string    `json:"model"`       // "iPhone 14", "Pixel 7"
	OSVersion  string    `json:"os_version"`  // "16.0", "13"
	AppVersion string    `json:"app_version"` // "1.2.3"
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// === Auth Config ===

// AuthConfig configures auth system behavior.
type AuthConfig struct {
	// JWT
	JWTSecret       string        `json:"jwt_secret"`
	AccessTokenTTL  time.Duration `json:"access_token_ttl"`  // default: 15min
	RefreshTokenTTL time.Duration `json:"refresh_token_ttl"` // default: 7d
	Issuer          string        `json:"issuer"`
	Audience        string        `json:"audience"` // optional; validated on JWTs when set

	// Session
	SessionStore      SessionStore `json:"-"`
	SessionCookieName string       `json:"session_cookie_name"` // default: "nitro_session"
	SessionSecure     bool         `json:"session_secure"`      // true in production
	SessionSameSite   string       `json:"session_same_site"`   // "Lax", "Strict", "None"

	// Password
	PasswordHasher PasswordHasher `json:"-"`

	// OAuth2
	OAuthProviders map[string]OAuthConfig `json:"oauth_providers,omitempty"`
}

// OAuthConfig configures an OAuth2/OIDC provider.
type OAuthConfig struct {
	ClientID     string         `json:"client_id"`
	ClientSecret string         `json:"client_secret"`
	RedirectURL  string         `json:"redirect_url"`
	Scopes       []string       `json:"scopes"`
	Endpoints    OAuthEndpoints `json:"endpoints"`
}

// OAuthEndpoints are the OAuth2 provider endpoints.
type OAuthEndpoints struct {
	AuthURL     string `json:"auth_url"`
	TokenURL    string `json:"token_url"`
	UserInfoURL string `json:"userinfo_url"`
	JWKSURL     string `json:"jwks_url"`
	LogoutURL   string `json:"logout_url"`
}

// === Pre-defined Errors ===

var (
	// ErrUnauthorized is a reusable allocation-free 401 error.
	ErrUnauthorized = &UnauthorizedError{Message: "unauthorized"}

	// ErrInvalidCredentials is a 401 error for invalid credentials.
	ErrInvalidCredentials = &UnauthorizedError{Message: "invalid credentials"}

	// ErrTokenExpired is a 401 error for expired tokens.
	ErrTokenExpired = &UnauthorizedError{Message: "token expired"}

	// ErrInvalidToken is a 401 error for invalid tokens.
	ErrInvalidToken = &UnauthorizedError{Message: "invalid token"}

	// ErrSessionExpired is a 401 error for expired sessions.
	ErrSessionExpired = &UnauthorizedError{Message: "session expired"}

	// ErrEmailAlreadyExists is a 409 error for a duplicate email.
	ErrEmailAlreadyExists = &ConflictError{Message: "email already exists"}

	// ErrWeakPassword is a 422 error for a weak password.
	ErrWeakPassword = &UnprocessableEntityError{Message: "password too weak"}

	// ErrEmailNotVerified is a 401 error returned when an OAuth2/OIDC
	// provider reports an unverified email for an account that already
	// exists locally — refusing to link/login prevents account takeover
	// by an attacker who controls an unverified address matching a
	// victim's email at the provider. See usecases.AuthUseCase.LoginOAuth.
	ErrEmailNotVerified = &UnauthorizedError{Message: "oauth email not verified by provider"}
)
