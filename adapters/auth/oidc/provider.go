// Package oidc provides a JWT TokenProvider that validates tokens via JWKS (RS256).
//
// Suitable for any OIDC IdP that exposes a JWKS endpoint. Role mapping flattens
// realm_access and resource_access claims into domain.Claims.Roles.
package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/ports/v2"
)

// Config configures the OIDC JWKS validator.
type Config struct {
	// Issuer expected in the "iss" claim. REQUIRED: without it a token minted
	// by any other tenant/IdP that shares the JWKS host would be accepted.
	Issuer string
	// JWKSURL is required unless derived elsewhere by the caller.
	JWKSURL string
	// Audience is the expected "aud" value (your API / client id). REQUIRED
	// unless AllowAnyAudience is set: skipping it lets a token issued for a
	// different application of the same IdP be replayed against this API
	// (confused deputy).
	Audience string
	// AllowAnyAudience explicitly disables the audience check. Only for IdPs
	// that cannot put your API in "aud"; prefer configuring Audience.
	AllowAnyAudience bool
	// ClientIDs used to extract roles from resource_access[client].roles.
	// If empty, roles from all resource_access entries are collected.
	ClientIDs []string
	// HTTPClient overrides the default 10s client.
	HTTPClient *http.Client
	// JWKSCacheTTL default 5m.
	JWKSCacheTTL time.Duration
}

// Provider validates OIDC access tokens via JWKS.
// GenerateAccessToken / GenerateRefreshToken are not supported (external IdP).
type Provider struct {
	cfg    Config
	client *http.Client
	cache  *jwksCache
}

var _ ports.TokenProvider = (*Provider)(nil)

type jwksCache struct {
	mu          sync.RWMutex
	keys        map[string]*rsa.PublicKey
	fetchedAt   time.Time
	ttl         time.Duration
	refreshOnce sync.Mutex

	// lastAttempt/minRetryInterval implement negative caching: without it,
	// every JWT with an unknown `kid` (trivial for an attacker to spam)
	// forces a fresh network round-trip to the IdP on every single
	// request, amplifying a small attacker request rate into sustained
	// load against the identity provider (DoS). Once a refresh has been
	// attempted, further lookups for a still-missing kid within this
	// window reuse the cached (negative) result instead of refetching.
	lastAttempt      time.Time
	minRetryInterval time.Duration
}

// New creates an OIDC TokenProvider.
func New(cfg Config) (*Provider, error) {
	if cfg.JWKSURL == "" {
		return nil, fmt.Errorf("oidc: JWKSURL is required")
	}
	if cfg.Issuer == "" {
		return nil, fmt.Errorf("oidc: Issuer is required (tokens from other issuers must be rejected)")
	}
	if cfg.Audience == "" && !cfg.AllowAnyAudience {
		return nil, fmt.Errorf("oidc: Audience is required (set Config.Audience, or AllowAnyAudience to opt out explicitly)")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	ttl := cfg.JWKSCacheTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Provider{
		cfg:    cfg,
		client: client,
		cache: &jwksCache{
			keys:             make(map[string]*rsa.PublicKey),
			ttl:              ttl,
			minRetryInterval: 30 * time.Second,
		},
	}, nil
}

// ValidateToken validates a JWT access token and returns flattened claims.
func (p *Provider) ValidateToken(_ context.Context, tokenStr string) (*domain.Claims, error) {
	claims := &oidcClaims{}
	// Only RSA signatures, and "exp" is mandatory: a token without an
	// expiry would otherwise be valid forever.
	token, err := jwt.ParseWithClaims(tokenStr, claims, p.keyFunc,
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return nil, domain.ErrUnauthorized
	}
	if claims.Issuer != p.cfg.Issuer {
		return nil, domain.ErrUnauthorized
	}
	if !p.cfg.AllowAnyAudience && !audienceContains(claims.Audience, p.cfg.Audience) {
		return nil, domain.ErrUnauthorized
	}

	userID := claims.Subject
	if userID == "" {
		userID = claims.Sub
	}
	email := claims.Email
	if email == "" {
		email = claims.PreferredUsername
	}

	return &domain.Claims{
		UserID:   userID,
		Email:    email,
		Roles:    flattenRoles(claims, p.cfg.ClientIDs),
		TenantID: claims.tenant(),
	}, nil
}

// GenerateAccessToken is not supported for external OIDC IdPs.
func (p *Provider) GenerateAccessToken(context.Context, *domain.User) (string, error) {
	return "", fmt.Errorf("oidc: GenerateAccessToken not supported")
}

// GenerateRefreshToken is not supported for external OIDC IdPs.
func (p *Provider) GenerateRefreshToken(context.Context, *domain.User) (string, error) {
	return "", fmt.Errorf("oidc: GenerateRefreshToken not supported")
}

// RefreshToken is not supported; use the IdP-specific client (e.g. Keycloak).
func (p *Provider) RefreshToken(context.Context, string) (*domain.TokenPair, error) {
	return nil, fmt.Errorf("oidc: RefreshToken not supported")
}

// RevokeToken is a no-op placeholder (use IdP logout).
func (p *Provider) RevokeToken(context.Context, string) error {
	return nil
}

func (p *Provider) keyFunc(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}
	kid, _ := token.Header["kid"].(string)
	return p.getKey(kid)
}

func (p *Provider) getKey(kid string) (*rsa.PublicKey, error) {
	p.cache.mu.RLock()
	key, ok := p.cache.keys[kid]
	expired := time.Since(p.cache.fetchedAt) > p.cache.ttl
	p.cache.mu.RUnlock()
	if ok && !expired {
		return key, nil
	}

	p.cache.refreshOnce.Lock()
	defer p.cache.refreshOnce.Unlock()

	p.cache.mu.RLock()
	key, ok = p.cache.keys[kid]
	expired = time.Since(p.cache.fetchedAt) > p.cache.ttl
	sinceLastAttempt := time.Since(p.cache.lastAttempt)
	p.cache.mu.RUnlock()
	if ok && !expired {
		return key, nil
	}

	// Negative-cache window: a kid that was missing on the last attempt
	// stays missing without re-hitting the IdP until minRetryInterval
	// elapses, even though the cache itself may be "expired" by ttl.
	if sinceLastAttempt < p.cache.minRetryInterval {
		return nil, fmt.Errorf("oidc: kid %q not found in JWKS", kid)
	}

	if err := p.refreshJWKS(); err != nil {
		return nil, err
	}

	p.cache.mu.Lock()
	p.cache.lastAttempt = time.Now()
	key, ok = p.cache.keys[kid]
	p.cache.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("oidc: kid %q not found in JWKS", kid)
	}
	return key, nil
}

func (p *Provider) refreshJWKS() error {
	resp, err := p.client.Get(p.cfg.JWKSURL)
	if err != nil {
		return fmt.Errorf("oidc: fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("oidc: JWKS status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&jwks); err != nil {
		return fmt.Errorf("oidc: decode JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := decodeJWK(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	p.cache.mu.Lock()
	p.cache.keys = keys
	p.cache.fetchedAt = time.Now()
	p.cache.mu.Unlock()
	return nil
}

func decodeJWK(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, err
	}
	var e int
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
}

// oidcClaims captures common Keycloak/OIDC JWT payload fields.
type oidcClaims struct {
	jwt.RegisteredClaims
	Sub               string         `json:"sub"`
	Email             string         `json:"email"`
	PreferredUsername string         `json:"preferred_username"`
	RealmAccess       map[string]any `json:"realm_access"`
	ResourceAccess    map[string]any `json:"resource_access"`

	// Tenant conventions (see domain.TenantClaimKeys).
	TenantID       string `json:"tenant_id"`
	TenantIDCamel  string `json:"tenantId"`
	Tid            string `json:"tid"`
	OrgID          string `json:"org_id"`
	OrganizationID string `json:"organization_id"`
}

func (c *oidcClaims) tenant() string {
	return domain.TenantFromClaims(func(k string) string {
		switch k {
		case "tenant_id":
			return c.TenantID
		case "tenantId":
			return c.TenantIDCamel
		case "tid":
			return c.Tid
		case "org_id":
			return c.OrgID
		case "organization_id":
			return c.OrganizationID
		}
		return ""
	})
}

func (c *oidcClaims) GetAudience() (jwt.ClaimStrings, error) {
	return c.Audience, nil
}

func flattenRoles(c *oidcClaims, clientIDs []string) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(role string) {
		if role == "" {
			return
		}
		if _, ok := seen[role]; ok {
			return
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}

	if c.RealmAccess != nil {
		if roles, ok := c.RealmAccess["roles"]; ok {
			for _, r := range toStrings(roles) {
				add(r)
			}
		}
	}

	if c.ResourceAccess == nil {
		return out
	}

	if len(clientIDs) == 0 {
		for _, v := range c.ResourceAccess {
			addClientRoles(v, add)
		}
		return out
	}
	for _, id := range clientIDs {
		if v, ok := c.ResourceAccess[id]; ok {
			addClientRoles(v, add)
		}
	}
	return out
}

func addClientRoles(v any, add func(string)) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	for _, r := range toStrings(m["roles"]) {
		add(r)
	}
}

func toStrings(v any) []string {
	switch x := v.(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, i := range x {
			if s, ok := i.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return x
	default:
		return nil
	}
}

func audienceContains(aud jwt.ClaimStrings, want string) bool {
	for _, a := range aud {
		if a == want {
			return true
		}
	}
	return false
}
