package keycloak

import (
	"context"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/adapters/auth/oidc/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/ports/v2"
)

// Provider validates Keycloak JWTs and refreshes via the token endpoint.
type Provider struct {
	oidc   *oidc.Provider
	tokens Client
	cfg    Config
}

var _ ports.TokenProvider = (*Provider)(nil)

// NewProvider builds a TokenProvider (validate via JWKS, refresh via Keycloak).
func NewProvider(cfg Config) (ports.TokenProvider, error) {
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
	return &Provider{
		oidc:   oidcProvider,
		tokens: NewClient(cfg),
		cfg:    cfg,
	}, nil
}

func (p *Provider) ValidateToken(ctx context.Context, token string) (*domain.Claims, error) {
	return p.oidc.ValidateToken(ctx, token)
}

func (p *Provider) GenerateAccessToken(context.Context, *domain.User) (string, error) {
	return "", fmt.Errorf("keycloak: GenerateAccessToken not supported; use Login")
}

func (p *Provider) GenerateRefreshToken(context.Context, *domain.User) (string, error) {
	return "", fmt.Errorf("keycloak: GenerateRefreshToken not supported; use Login")
}

func (p *Provider) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	tr, err := p.tokens.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	return &domain.TokenPair{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresIn:    tr.ExpiresIn,
		TokenType:    tr.TokenType,
	}, nil
}

func (p *Provider) RevokeToken(ctx context.Context, refreshToken string) error {
	return p.tokens.Logout(ctx, refreshToken)
}
