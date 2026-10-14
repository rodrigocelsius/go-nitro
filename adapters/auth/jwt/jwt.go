package jwt

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	nitrouuid "github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

// MinSecretLen is the minimum JWTSecret length accepted by New. 32 bytes
// (256 bits) matches the HS256 key size recommended by RFC 2104/NIST —
// generate with `openssl rand -hex 32`. The previous minimum of 16 was
// enough to pass validation with a short, memorable human-chosen phrase
// that has far less real entropy than its length suggests (see vuln.md
// V-25).
const MinSecretLen = 32

type Provider struct {
	secret          []byte
	issuer          string
	audience        string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration

	storeMu sync.Mutex
	store   RevocationStore // revoked JTIs (guarded by its own locking)
}

// New creates a new JWT token provider.
// Returns an error if JWTSecret is shorter than MinSecretLen.
func New(cfg domain.AuthConfig) (*Provider, error) {
	if len(cfg.JWTSecret) < MinSecretLen {
		return nil, fmt.Errorf("jwt: JWTSecret must be at least %d characters", MinSecretLen)
	}
	// Documented defaults (domain.AuthConfig): a zero TTL used to mint
	// tokens that expired at issuance.
	access, refresh := cfg.AccessTokenTTL, cfg.RefreshTokenTTL
	if access <= 0 {
		access = DefaultAccessTokenTTL
	}
	if refresh <= 0 {
		refresh = DefaultRefreshTokenTTL
	}
	return &Provider{
		secret:          []byte(cfg.JWTSecret),
		issuer:          cfg.Issuer,
		audience:        cfg.Audience,
		accessTokenTTL:  access,
		refreshTokenTTL: refresh,
		store:           newMemoryStore(),
	}, nil
}

// Default token lifetimes when AuthConfig leaves them zero.
const (
	DefaultAccessTokenTTL  = 15 * time.Minute
	DefaultRefreshTokenTTL = 7 * 24 * time.Hour
)

// AccessTokenTTL reports the access-token lifetime (expires_in).
func (p *Provider) AccessTokenTTL() time.Duration { return p.accessTokenTTL }

// MustNew is like New but panics on invalid config.
func MustNew(cfg domain.AuthConfig) *Provider {
	p, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return p
}

// claims builds the base claim set shared by access and refresh tokens.
func (p *Provider) claims(user *domain.User, ttl time.Duration, typ string) jwt.MapClaims {
	now := time.Now()
	c := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"roles": user.Roles,
		"iat":   now.Unix(),
		"exp":   now.Add(ttl).Unix(),
		"jti":   nitrouuid.NewString(),
		"type":  typ,
	}
	if user.TenantID != "" {
		c["tenant_id"] = user.TenantID
	}
	if p.issuer != "" {
		c["iss"] = p.issuer
	}
	if p.audience != "" {
		c["aud"] = p.audience
	}
	return c
}

// GenerateAccessToken generates an access JWT.
func (p *Provider) GenerateAccessToken(ctx context.Context, user *domain.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, p.claims(user, p.accessTokenTTL, "access"))
	return token.SignedString(p.secret)
}

// GenerateRefreshToken generates a refresh JWT.
func (p *Provider) GenerateRefreshToken(ctx context.Context, user *domain.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, p.claims(user, p.refreshTokenTTL, "refresh"))
	return token.SignedString(p.secret)
}

// parse validates a signed JWT: HS256 only, expiration required, issuer and
// audience checked when configured. Returns ErrUnauthorized on any failure.
func (p *Provider) parse(tokenStr string) (jwt.MapClaims, error) {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
	}
	if p.issuer != "" {
		opts = append(opts, jwt.WithIssuer(p.issuer))
	}
	if p.audience != "" {
		opts = append(opts, jwt.WithAudience(p.audience))
	}
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return p.secret, nil
	}, opts...)
	if err != nil || !token.Valid {
		return nil, domain.ErrUnauthorized
	}
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	return mapClaims, nil
}

// ValidateToken validates a JWT and returns the claims.
func (p *Provider) ValidateToken(ctx context.Context, tokenStr string) (*domain.Claims, error) {
	mapClaims, err := p.parse(tokenStr)
	if err != nil {
		return nil, err
	}

	if typ, _ := mapClaims["type"].(string); typ != "access" {
		return nil, domain.ErrUnauthorized
	}

	if jti, _ := mapClaims["jti"].(string); jti != "" && p.isRevoked(ctx, jti) {
		return nil, domain.ErrUnauthorized
	}

	userID, _ := mapClaims["sub"].(string)
	if userID == "" {
		return nil, domain.ErrUnauthorized
	}

	return &domain.Claims{
		UserID:   userID,
		Email:    claimString(mapClaims, "email"),
		Roles:    toStringSlice(mapClaims["roles"]),
		TenantID: domain.TenantFromClaims(func(k string) string { return claimString(mapClaims, k) }),
	}, nil
}

// RefreshToken validates a refresh token, revokes it, and returns a new token pair.
func (p *Provider) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	mapClaims, err := p.parse(refreshToken)
	if err != nil {
		return nil, err
	}

	if typ, _ := mapClaims["type"].(string); typ != "refresh" {
		return nil, domain.ErrUnauthorized
	}

	jti, _ := mapClaims["jti"].(string)
	if jti != "" && p.isRevoked(ctx, jti) {
		return nil, domain.ErrUnauthorized
	}

	userID, _ := mapClaims["sub"].(string)
	if userID == "" {
		return nil, domain.ErrUnauthorized
	}

	// Rotate: revoke old refresh JTI
	if jti != "" {
		exp := time.Now().Add(p.refreshTokenTTL)
		if expClaim, err := mapClaims.GetExpirationTime(); err == nil && expClaim != nil {
			exp = expClaim.Time
		}
		p.revoke(ctx, jti, exp)
	}

	user := &domain.User{
		ID:       userID,
		Email:    claimString(mapClaims, "email"),
		Roles:    toStringSlice(mapClaims["roles"]),
		TenantID: domain.TenantFromClaims(func(k string) string { return claimString(mapClaims, k) }),
	}

	accessToken, err := p.GenerateAccessToken(ctx, user)
	if err != nil {
		return nil, err
	}
	refreshTokenNew, err := p.GenerateRefreshToken(ctx, user)
	if err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenNew,
		ExpiresIn:    int(p.accessTokenTTL.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// RevokeToken blacklists the token JTI until its natural expiry.
func (p *Provider) RevokeToken(ctx context.Context, tokenStr string) error {
	mapClaims, err := p.parse(tokenStr)
	if err != nil {
		return nil // already invalid
	}
	jti, _ := mapClaims["jti"].(string)
	if jti == "" {
		return nil
	}
	exp := time.Now().Add(p.refreshTokenTTL)
	if expClaim, err := mapClaims.GetExpirationTime(); err == nil && expClaim != nil {
		exp = expClaim.Time
	}
	p.revoke(ctx, jti, exp)
	return nil
}

func (p *Provider) revoke(ctx context.Context, jti string, exp time.Time) {
	_ = p.store.Revoke(ctx, jti, exp)
}

func (p *Provider) isRevoked(ctx context.Context, jti string) bool {
	revoked, err := p.store.Revoked(ctx, jti)
	if err != nil {
		// Fail closed: an unreachable revocation store must not silently
		// accept revoked tokens.
		return true
	}
	return revoked
}

func claimString(m jwt.MapClaims, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]any); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	if arr, ok := v.([]string); ok {
		return arr
	}
	return nil
}
