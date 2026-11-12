package jwt

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"strings"
	"testing"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

var keyTestUser = &domain.User{ID: "u1", Email: "a@b.c", Roles: []string{"admin"}}

func mustEC(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestNewWithKeys_SignsAndValidatesEachAlgorithm(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	_, edKey, _ := ed25519.GenerateKey(rand.Reader)
	for alg, key := range map[string]SigningKey{
		"RS256": {ID: "rsa-1", Key: rsaKey},
		"ES256": {ID: "ec-1", Key: mustEC(t)},
		"EdDSA": {ID: "ed-1", Key: edKey},
	} {
		p, err := NewWithKeys(domain.AuthConfig{}, Keys{Signing: key})
		if err != nil {
			t.Fatalf("%s: %v", alg, err)
		}
		tok, err := p.GenerateAccessToken(context.Background(), keyTestUser)
		if err != nil {
			t.Fatalf("%s: %v", alg, err)
		}
		parsed, _, _ := gojwt.NewParser().ParseUnverified(tok, gojwt.MapClaims{})
		if parsed.Method.Alg() != alg || parsed.Header["kid"] != key.ID {
			t.Errorf("%s: header alg=%s kid=%v", alg, parsed.Method.Alg(), parsed.Header["kid"])
		}
		claims, err := p.ValidateToken(context.Background(), tok)
		if err != nil || claims.UserID != "u1" {
			t.Errorf("%s: validate: %v %+v", alg, err, claims)
		}
	}
}

func TestNewWithKeys_RejectsWeakAndUnsupportedKeys(t *testing.T) {
	weak, _ := rsa.GenerateKey(rand.Reader, 1024)
	p384, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	for name, k := range map[string]SigningKey{
		"weak rsa": {ID: "k", Key: weak},
		"p-384":    {ID: "k", Key: p384},
		"no kid":   {Key: mustEC(t)},
		"nil key":  {ID: "k"},
	} {
		if _, err := NewWithKeys(domain.AuthConfig{}, Keys{Signing: k}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRotation_OldTokensStayValidUntilRetired(t *testing.T) {
	ctx := context.Background()
	p, err := NewWithKeys(domain.AuthConfig{}, Keys{Signing: SigningKey{ID: "2026-01", Key: mustEC(t)}})
	if err != nil {
		t.Fatal(err)
	}
	old, _ := p.GenerateAccessToken(ctx, keyTestUser)

	if err := p.RotateSigningKey(SigningKey{ID: "2026-02", Key: mustEC(t)}); err != nil {
		t.Fatal(err)
	}
	fresh, _ := p.GenerateAccessToken(ctx, keyTestUser)
	for name, tok := range map[string]string{"old": old, "new": fresh} {
		if _, err := p.ValidateToken(ctx, tok); err != nil {
			t.Errorf("%s token after rotation: %v", name, err)
		}
	}

	if err := p.RetireKey("2026-02"); err == nil {
		t.Error("retiring the current signing key must fail")
	}
	if err := p.RetireKey("2026-01"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ValidateToken(ctx, old); err == nil {
		t.Error("token of a retired key must be rejected")
	}
	if _, err := p.ValidateToken(ctx, fresh); err != nil {
		t.Errorf("current key: %v", err)
	}
}

// A token signed with HS256 using the public key bytes as the secret (the
// classic algorithm-confusion attack), or by an unknown key reusing a known
// kid, must be rejected.
func TestAsymmetric_RejectsForgedTokens(t *testing.T) {
	ctx := context.Background()
	key := mustEC(t)
	p, _ := NewWithKeys(domain.AuthConfig{}, Keys{Signing: SigningKey{ID: "k1", Key: key}})
	claims := p.claims(keyTestUser, p.accessTokenTTL, "access")

	hs := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	hs.Header["kid"] = "k1"
	forged, _ := hs.SignedString(elliptic.MarshalCompressed(elliptic.P256(), key.X, key.Y))
	if _, err := p.ValidateToken(ctx, forged); err == nil {
		t.Error("HS256 token accepted by an asymmetric provider")
	}

	other := gojwt.NewWithClaims(gojwt.SigningMethodES256, claims)
	other.Header["kid"] = "k1"
	forged, _ = other.SignedString(mustEC(t))
	if _, err := p.ValidateToken(ctx, forged); err == nil {
		t.Error("token signed by another key accepted")
	}

	noKid := gojwt.NewWithClaims(gojwt.SigningMethodES256, claims)
	forged, _ = noKid.SignedString(key)
	if _, err := p.ValidateToken(ctx, forged); err == nil {
		t.Error("token without kid accepted")
	}
}

// A verify-only instance (public key only) validates but cannot be built
// without a signing key of its own; the JWKS never carries private material.
func TestJWKS_PublishesPublicKeysOnly(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	prev := mustEC(t)
	p, err := NewWithKeys(domain.AuthConfig{}, Keys{
		Signing: SigningKey{ID: "rsa-1", Key: rsaKey},
		Verify:  map[string]crypto.PublicKey{"ec-0": &prev.PublicKey},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := p.JWKS()
	if err != nil {
		t.Fatal(err)
	}
	var set struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(raw, &set); err != nil {
		t.Fatal(err)
	}
	if len(set.Keys) != 2 || set.Keys[0]["kid"] != "ec-0" || set.Keys[1]["kid"] != "rsa-1" {
		t.Fatalf("keys = %s", raw)
	}
	if set.Keys[0]["kty"] != "EC" || set.Keys[1]["kty"] != "RSA" || set.Keys[1]["n"] == "" {
		t.Errorf("keys = %s", raw)
	}
	for _, private := range []string{`"d"`, `"p"`, `"q"`} {
		if strings.Contains(string(raw), private) {
			t.Errorf("JWKS leaks private parameter %s", private)
		}
	}

	hs, _ := New(domain.AuthConfig{JWTSecret: strings.Repeat("s", 32)})
	if raw, _ := hs.JWKS(); string(raw) != `{"keys":[]}` {
		t.Errorf("HS256 JWKS = %s", raw)
	}
	if hs.Asymmetric() || !p.Asymmetric() {
		t.Error("Asymmetric()")
	}
	if err := hs.RotateSigningKey(SigningKey{ID: "x", Key: prev}); err == nil {
		t.Error("RotateSigningKey on an HS256 provider must fail")
	}
}

func TestUsesMemoryRevocation(t *testing.T) {
	p, _ := New(domain.AuthConfig{JWTSecret: strings.Repeat("s", 32)})
	if !p.UsesMemoryRevocation() {
		t.Error("default store is in-memory")
	}
	p.SetRevocationStore(newMemoryStore())
	if p.UsesMemoryRevocation() {
		t.Error("a custom store is not the in-memory default")
	}
	p.SetRevocationStore(nil)
	if !p.UsesMemoryRevocation() {
		t.Error("nil restores the in-memory default")
	}
}
