package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestProvider_ValidateToken(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	jwks := map[string]any{
		"keys": []map[string]string{
			{
				"kid": "test-kid",
				"kty": "RSA",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(priv.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.E)).Bytes()),
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)

	p, err := New(Config{
		Issuer:           "https://auth.example/realms/demo",
		JWKSURL:          srv.URL,
		AllowAnyAudience: true,
		ClientIDs:        []string{"scheduler-api"},
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":                "user-1",
		"iss":                "https://auth.example/realms/demo",
		"email":              "a@b.com",
		"iat":                now.Unix(),
		"exp":                now.Add(time.Hour).Unix(),
		"realm_access":       map[string]any{"roles": []any{"admin"}},
		"resource_access":    map[string]any{"scheduler-api": map[string]any{"roles": []any{"super_admin"}}},
		"preferred_username": "ana",
	})
	token.Header["kid"] = "test-kid"
	signed, err := token.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := p.ValidateToken(context.Background(), signed)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "user-1" || claims.Email != "a@b.com" {
		t.Fatalf("claims %#v", claims)
	}
	if !contains(claims.Roles, "admin") || !contains(claims.Roles, "super_admin") {
		t.Fatalf("roles %#v", claims.Roles)
	}
}

func TestProvider_RejectWrongIssuer(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	n := base64.RawURLEncoding.EncodeToString(priv.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.E)).Bytes())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{{"kid": "k", "kty": "RSA", "n": n, "e": e}},
		})
	}))
	t.Cleanup(srv.Close)

	p, err := New(Config{Issuer: "https://expected", JWKSURL: srv.URL, AllowAnyAudience: true})
	if err != nil {
		t.Fatal(err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "u",
		"iss": "https://other",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	token.Header["kid"] = "k"
	signed, _ := token.SignedString(priv)

	_, err = p.ValidateToken(context.Background(), signed)
	if err != domain.ErrUnauthorized {
		t.Fatalf("want unauthorized, got %v", err)
	}
}

func TestProvider_UnknownKidNegativeCache(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	n := base64.RawURLEncoding.EncodeToString(priv.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.E)).Bytes())

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{{"kid": "known", "kty": "RSA", "n": n, "e": e}},
		})
	}))
	t.Cleanup(srv.Close)

	p, err := New(Config{Issuer: "https://x", JWKSURL: srv.URL, AllowAnyAudience: true})
	if err != nil {
		t.Fatal(err)
	}

	// Repeated lookups for a kid that never exists in the JWKS should not
	// hammer the IdP once the negative-cache window kicks in — an
	// attacker can send arbitrary random `kid`s cheaply, so each one
	// must not cost a network round-trip.
	for i := 0; i < 10; i++ {
		if _, err := p.getKey("unknown-kid"); err == nil {
			t.Fatal("expected error for unknown kid")
		}
	}
	if hits > 1 {
		t.Fatalf("expected at most 1 JWKS fetch due to negative caching, got %d", hits)
	}
	if hits == 0 {
		t.Fatal("expected at least 1 JWKS fetch on first lookup")
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func newTestOIDC(t *testing.T, cfg Config) (*Provider, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	n := base64.RawURLEncoding.EncodeToString(priv.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.E)).Bytes())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kid": "k", "kty": "RSA", "n": n, "e": e}}})
	}))
	t.Cleanup(srv.Close)
	cfg.JWKSURL = srv.URL
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return p, priv
}

func sign(t *testing.T, priv *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "k"
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Regression: iss/aud/exp used to be optional.
func TestNew_RequiresIssuerAndAudience(t *testing.T) {
	if _, err := New(Config{JWKSURL: "http://x"}); err == nil {
		t.Fatal("empty Issuer must be rejected")
	}
	if _, err := New(Config{JWKSURL: "http://x", Issuer: "https://i"}); err == nil {
		t.Fatal("empty Audience must be rejected unless AllowAnyAudience")
	}
	if _, err := New(Config{JWKSURL: "http://x", Issuer: "https://i", AllowAnyAudience: true}); err != nil {
		t.Fatalf("explicit opt-out should work: %v", err)
	}
}

func TestValidateToken_RejectsMissingExpAndWrongAudience(t *testing.T) {
	p, priv := newTestOIDC(t, Config{Issuer: "https://i", Audience: "my-api"})
	good := jwt.MapClaims{"sub": "u", "iss": "https://i", "aud": "my-api", "exp": time.Now().Add(time.Hour).Unix()}
	if _, err := p.ValidateToken(context.Background(), sign(t, priv, good)); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	noExp := jwt.MapClaims{"sub": "u", "iss": "https://i", "aud": "my-api"}
	if _, err := p.ValidateToken(context.Background(), sign(t, priv, noExp)); err == nil {
		t.Fatal("token without exp must be rejected")
	}
	wrongAud := jwt.MapClaims{"sub": "u", "iss": "https://i", "aud": "other-app", "exp": time.Now().Add(time.Hour).Unix()}
	if _, err := p.ValidateToken(context.Background(), sign(t, priv, wrongAud)); err == nil {
		t.Fatal("token for another audience must be rejected")
	}
}
