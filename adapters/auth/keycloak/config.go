// Package keycloak provides Keycloak OIDC token client, admin API and JWT provider.
package keycloak

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rodrigocelsius/go-nitro/adapters/auth/oidc/v2"
	"github.com/rodrigocelsius/go-nitro/ports/v2"
)

// Config holds Keycloak connection settings.
type Config struct {
	BaseURL      string
	Realm        string
	ClientID     string
	ClientSecret string
	// Audience optional JWT aud check (often the client id).
	Audience     string
	HTTPClient   *http.Client
	JWKSCacheTTL time.Duration
}

// JWKSEndpoint derives the JWKS URL.
func (c Config) JWKSEndpoint() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", trimSlash(c.BaseURL), c.Realm)
}

// TokenEndpoint derives the token URL.
func (c Config) TokenEndpoint() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", trimSlash(c.BaseURL), c.Realm)
}

// LogoutEndpoint derives the logout URL.
func (c Config) LogoutEndpoint() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/logout", trimSlash(c.BaseURL), c.Realm)
}

// Issuer derives the realm issuer.
func (c Config) Issuer() string {
	return fmt.Sprintf("%s/realms/%s", trimSlash(c.BaseURL), c.Realm)
}

// AdminUsersEndpoint derives admin users URL.
func (c Config) AdminUsersEndpoint() string {
	return fmt.Sprintf("%s/admin/realms/%s/users", trimSlash(c.BaseURL), c.Realm)
}

// AdminRolesEndpoint derives admin roles URL.
func (c Config) AdminRolesEndpoint() string {
	return fmt.Sprintf("%s/admin/realms/%s/roles", trimSlash(c.BaseURL), c.Realm)
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func httpClient(cfg Config) *http.Client {
	if cfg.HTTPClient != nil {
		return cfg.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// Service bundles TokenProvider + token Client + AdminClient.
type Service struct {
	cfg      Config
	provider ports.TokenProvider
	tokens   Client
	admin    AdminClient
}

// New creates a Keycloak service (validator + client + admin).
func New(cfg Config) (*Service, error) {
	if cfg.BaseURL == "" || cfg.Realm == "" {
		return nil, fmt.Errorf("keycloak: BaseURL and Realm are required")
	}
	oidcProvider, err := oidc.New(oidc.Config{
		Issuer:   cfg.Issuer(),
		JWKSURL:  cfg.JWKSEndpoint(),
		Audience: cfg.Audience,
		// Keycloak access tokens often carry aud="account" rather than the
		// client id; keep working when Audience is unset, but set it.
		AllowAnyAudience: cfg.Audience == "",
		ClientIDs:        []string{cfg.ClientID},
		HTTPClient:       httpClient(cfg),
		JWKSCacheTTL:     cfg.JWKSCacheTTL,
	})
	if err != nil {
		return nil, err
	}
	tok := NewClient(cfg)
	adm := NewAdmin(cfg)
	return &Service{
		cfg:      cfg,
		provider: &Provider{oidc: oidcProvider, tokens: tok, cfg: cfg},
		tokens:   tok,
		admin:    adm,
	}, nil
}

// Provider returns a ports.TokenProvider for JWTMiddleware.
func (s *Service) Provider() ports.TokenProvider { return s.provider }

// Tokens returns the password/refresh/logout client.
func (s *Service) Tokens() Client { return s.tokens }

// Admin returns the Admin REST API client.
func (s *Service) Admin() AdminClient { return s.admin }
