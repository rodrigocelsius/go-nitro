package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func testCfg() domain.AuthConfig {
	return domain.AuthConfig{
		JWTSecret:       "test-secret-at-least-32-characters-long",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "test-issuer",
	}
}

func TestNew_RejectsShortSecret(t *testing.T) {
	_, err := New(domain.AuthConfig{JWTSecret: "short"})
	if err == nil {
		t.Fatal("expected error for short JWTSecret")
	}
}

func TestProvider_GenerateAccessToken(t *testing.T) {
	p, err := New(testCfg())
	if err != nil {
		t.Fatal(err)
	}

	user := &domain.User{
		ID:    "user-123",
		Email: "test@example.com",
		Roles: []string{"admin"},
	}

	token, err := p.GenerateAccessToken(context.Background(), user)
	if err != nil {
		t.Fatalf("GenerateAccessToken error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := p.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateToken error: %v", err)
	}
	if claims.UserID != user.ID {
		t.Fatalf("expected user ID %q, got %q", user.ID, claims.UserID)
	}
	if claims.Email != user.Email {
		t.Fatalf("expected email %q, got %q", user.Email, claims.Email)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "admin" {
		t.Fatalf("expected roles [admin], got %v", claims.Roles)
	}
}

func TestProvider_GenerateAccessToken_IncludesTenantID(t *testing.T) {
	p, err := New(testCfg())
	if err != nil {
		t.Fatal(err)
	}

	user := &domain.User{
		ID:       "user-123",
		Email:    "test@example.com",
		TenantID: "tenant-abc",
	}

	token, err := p.GenerateAccessToken(context.Background(), user)
	if err != nil {
		t.Fatalf("GenerateAccessToken error: %v", err)
	}

	claims, err := p.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateToken error: %v", err)
	}
	if claims.TenantID != user.TenantID {
		t.Fatalf("expected tenant ID %q, got %q", user.TenantID, claims.TenantID)
	}
}

func TestProvider_GenerateRefreshToken(t *testing.T) {
	p := MustNew(testCfg())

	user := &domain.User{
		ID:    "user-123",
		Email: "test@example.com",
	}

	token, err := p.GenerateRefreshToken(context.Background(), user)
	if err != nil {
		t.Fatalf("GenerateRefreshToken error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestProvider_ValidateToken_Invalid(t *testing.T) {
	p := MustNew(testCfg())

	_, err := p.ValidateToken(context.Background(), "invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestProvider_RefreshToken(t *testing.T) {
	p := MustNew(testCfg())

	user := &domain.User{
		ID:    "user-123",
		Email: "test@example.com",
		Roles: []string{"user"},
	}

	refreshToken, err := p.GenerateRefreshToken(context.Background(), user)
	if err != nil {
		t.Fatalf("GenerateRefreshToken error: %v", err)
	}

	pair, err := p.RefreshToken(context.Background(), refreshToken)
	if err != nil {
		t.Fatalf("RefreshToken error: %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if pair.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token")
	}
	if pair.TokenType != "Bearer" {
		t.Fatalf("expected token type Bearer, got %q", pair.TokenType)
	}

	// Old refresh must be revoked (rotation)
	_, err = p.RefreshToken(context.Background(), refreshToken)
	if err == nil {
		t.Fatal("expected error reusing revoked refresh token")
	}

	claims, err := p.ValidateToken(context.Background(), pair.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Email != user.Email {
		t.Fatalf("email not preserved on refresh: %q", claims.Email)
	}
}

func TestProvider_RevokeToken(t *testing.T) {
	p := MustNew(testCfg())
	user := &domain.User{ID: "u1", Email: "a@b.c"}
	tok, err := p.GenerateAccessToken(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.RevokeToken(context.Background(), tok); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ValidateToken(context.Background(), tok); err == nil {
		t.Fatal("expected revoked access token to fail validation")
	}
}

func TestProvider_ValidateToken_RejectsWrongAlgorithm(t *testing.T) {
	p := MustNew(testCfg())
	// Token signed with HS384 must be rejected even with a valid HMAC signature.
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, jwt.MapClaims{
		"sub": "u1", "type": "access", "exp": time.Now().Add(time.Hour).Unix(),
	})
	hs384, err := token.SignedString([]byte("test-secret-16chars"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ValidateToken(context.Background(), hs384); err == nil {
		t.Fatal("expected HS384 token to be rejected")
	}
}

func TestProvider_ValidateToken_RejectsWrongIssuer(t *testing.T) {
	cfg := testCfg()
	p := MustNew(cfg)
	other := MustNew(domain.AuthConfig{JWTSecret: cfg.JWTSecret, AccessTokenTTL: cfg.AccessTokenTTL, Issuer: "evil-issuer"})
	user := &domain.User{ID: "u1"}
	tok, err := other.GenerateAccessToken(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ValidateToken(context.Background(), tok); err == nil {
		t.Fatal("expected token from a different issuer to be rejected")
	}
}

func TestProvider_ValidateToken_Audience(t *testing.T) {
	cfg := testCfg()
	cfg.Audience = "my-api"
	p := MustNew(cfg)

	user := &domain.User{ID: "u1", Email: "a@b.c"}
	tok, err := p.GenerateAccessToken(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ValidateToken(context.Background(), tok); err != nil {
		t.Fatalf("expected aud-bearing token to validate: %v", err)
	}

	noAud := MustNew(testCfg())
	if _, err := noAud.ValidateToken(context.Background(), tok); err != nil {
		t.Fatalf("aud claim is ignored when no audience is configured: %v", err)
	}

	// Token without aud must fail when audience is required.
	bare := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "u1", "iss": cfg.Issuer, "type": "access",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := bare.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ValidateToken(context.Background(), signed); err == nil {
		t.Fatal("expected token without aud to be rejected when audience configured")
	}
}

func TestProvider_ValidateToken_RequiresExp(t *testing.T) {
	p := MustNew(testCfg())
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "u1", "iss": "test-issuer", "type": "access",
	})
	signed, err := token.SignedString([]byte("test-secret-16chars"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ValidateToken(context.Background(), signed); err == nil {
		t.Fatal("expected token without exp to be rejected")
	}
}

func TestProvider_ValidateToken_MissingSub(t *testing.T) {
	p := MustNew(testCfg())
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "test-issuer", "type": "access",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte("test-secret-16chars"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ValidateToken(context.Background(), signed); err == nil {
		t.Fatal("expected token without sub to be rejected")
	}
}

type stubStore struct {
	revoked map[string]bool
	err     error
}

func (s *stubStore) Revoke(_ context.Context, jti string, _ time.Time) error {
	if s.err != nil {
		return s.err
	}
	s.revoked[jti] = true
	return nil
}

func (s *stubStore) Revoked(_ context.Context, jti string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.revoked[jti], nil
}

func TestProvider_PluggableRevocationStore(t *testing.T) {
	p := MustNew(testCfg())
	store := &stubStore{revoked: map[string]bool{}}
	p.SetRevocationStore(store)

	user := &domain.User{ID: "u1"}
	tok, err := p.GenerateAccessToken(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.RevokeToken(context.Background(), tok); err != nil {
		t.Fatal(err)
	}
	if len(store.revoked) == 0 {
		t.Fatal("custom store was not used for revocation")
	}
	if _, err := p.ValidateToken(context.Background(), tok); err == nil {
		t.Fatal("expected revoked token to fail validation via custom store")
	}
}

func TestProvider_RevocationFailClosed(t *testing.T) {
	p := MustNew(testCfg())
	p.SetRevocationStore(&stubStore{err: context.DeadlineExceeded})

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "u1", "iss": "test-issuer", "jti": "j-1", "type": "access",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte("test-secret-16chars"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ValidateToken(context.Background(), signed); err == nil {
		t.Fatal("expected fail-closed when revocation store errors")
	}
}
