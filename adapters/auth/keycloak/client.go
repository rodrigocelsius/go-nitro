package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// TokenResponse is the OIDC token endpoint payload.
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
}

// Client manages Keycloak token lifecycle (password grant).
type Client interface {
	Login(ctx context.Context, username, password string) (*TokenResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error)
	Logout(ctx context.Context, refreshToken string) error
}

type tokenClient struct {
	cfg    Config
	client *http.Client
}

// NewClient creates a Keycloak token client.
func NewClient(cfg Config) Client {
	return &tokenClient{cfg: cfg, client: httpClient(cfg)}
}

func (c *tokenClient) Login(ctx context.Context, username, password string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("username", username)
	form.Set("password", password)
	form.Set("scope", "openid profile email")
	return c.postToken(ctx, form)
}

func (c *tokenClient) Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("refresh_token", refreshToken)
	return c.postToken(ctx, form)
}

func (c *tokenClient) Logout(ctx context.Context, refreshToken string) error {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.LogoutEndpoint(), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("keycloak logout: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("keycloak logout status: %d", resp.StatusCode)
	}
	return nil
}

func (c *tokenClient) postToken(ctx context.Context, form url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenEndpoint(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keycloak token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var tr TokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
			return nil, fmt.Errorf("keycloak token decode: %w", err)
		}
		if tr.TokenType == "" {
			tr.TokenType = "Bearer"
		}
		return &tr, nil
	}

	var errResp struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&errResp)

	// Keycloak uses 400 invalid_grant for invalid refresh/login (not 401).
	if resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusBadRequest ||
		errResp.Error == "invalid_grant" ||
		errResp.Error == "unauthorized_client" {
		msg := errResp.ErrorDescription
		if msg == "" {
			msg = errResp.Error
		}
		if msg == "" {
			msg = "invalid credentials"
		}
		if form.Get("grant_type") == "refresh_token" && msg == "invalid credentials" {
			msg = "invalid or expired refresh token"
		}
		return nil, domainUnauthorized(msg)
	}

	return nil, fmt.Errorf("keycloak token status %d: %s (%s)", resp.StatusCode, errResp.Error, errResp.ErrorDescription)
}
