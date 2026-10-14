package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	goauth2 "golang.org/x/oauth2"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// UserFunc fetches user info from the OAuth2 provider using the token.
type UserFunc func(ctx context.Context, token *goauth2.Token) (*domain.User, error)

// Provider wraps an OAuth2 configuration and user info fetcher.
type Provider struct {
	name      string
	config    *goauth2.Config
	endpoints domain.OAuthEndpoints
	userFunc  UserFunc
}

// New creates a new OAuth2 provider.
func New(name string, cfg domain.OAuthConfig, userFunc UserFunc) *Provider {
	return &Provider{
		name: name,
		config: &goauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint: goauth2.Endpoint{
				AuthURL:  cfg.Endpoints.AuthURL,
				TokenURL: cfg.Endpoints.TokenURL,
			},
		},
		endpoints: cfg.Endpoints,
		userFunc:  userFunc,
	}
}

// Name returns the provider name (e.g., "google", "github").
func (p *Provider) Name() string {
	return p.name
}

// AuthURL returns the authorization URL to redirect the user to.
// The state parameter is used for CSRF protection.
func (p *Provider) AuthURL(state string) string {
	return p.config.AuthCodeURL(state)
}

// TokenExchange exchanges an authorization code for an OAuth2 token.
func (p *Provider) TokenExchange(ctx context.Context, code string) (*goauth2.Token, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth2 %s: exchange: %w", p.name, err)
	}
	return token, nil
}

// FetchUser retrieves user info from the provider using the token.
func (p *Provider) FetchUser(ctx context.Context, token *goauth2.Token) (*domain.User, error) {
	user, err := p.userFunc(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("oauth2 %s: fetch user: %w", p.name, err)
	}
	return user, nil
}

// Callback processes the OAuth2 callback: exchanges code for token, fetches user.
func (p *Provider) Callback(ctx context.Context, code string) (*domain.User, error) {
	token, err := p.TokenExchange(ctx, code)
	if err != nil {
		return nil, err
	}

	user, err := p.FetchUser(ctx, token)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// AuthURLPKCE is like AuthURL but adds an S256 PKCE code_challenge derived
// from verifier — required for public clients (SPA/mobile, no confidential
// client secret) so an attacker who intercepts the authorization `code`
// (e.g. via a leaky redirect/referrer or a malicious app registering the
// same custom URI scheme) cannot exchange it for a token without also
// knowing verifier. Pair with CallbackPKCE, presenting the same verifier
// at the token exchange step (RFC 7636).
func (p *Provider) AuthURLPKCE(state, verifier string) string {
	return p.config.AuthCodeURL(state, goauth2.S256ChallengeOption(verifier))
}

// CallbackPKCE is like Callback but presents the PKCE code_verifier during
// the token exchange, as required when the authorization URL was built via
// AuthURLPKCE.
func (p *Provider) CallbackPKCE(ctx context.Context, code, verifier string) (*domain.User, error) {
	token, err := p.config.Exchange(ctx, code, goauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("oauth2 %s: exchange: %w", p.name, err)
	}

	user, err := p.FetchUser(ctx, token)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// OAuthConfig returns the underlying golang.org/x/oauth2.Config.
func (p *Provider) OAuthConfig() *goauth2.Config {
	return p.config
}

// === HTTP helpers for providers ===

// fetchJSON performs a GET request and decodes the JSON response into dest.
func fetchJSON(ctx context.Context, url, accessToken string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}
