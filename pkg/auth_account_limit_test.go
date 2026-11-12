package nitro

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Failed logins are counted per account, whatever the client IP: after the
// limit the account answers 429 even with the right password, and a
// different account is not affected.
func TestAuth_AccountRateLimit(t *testing.T) {
	auth := NewAuth(AuthConfig{JWTSecret: hardeningSecret}).WithMemoryStore().
		WithRateLimit(0, 0).WithAccountRateLimit(3, time.Minute)
	app := hardeningApp(auth)
	for _, email := range []string{"ana@example.com", "bia@example.com"} {
		if code, _ := hdo(t, app, "POST", "/auth/register", `{"email":"`+email+`","password":"password123"}`); code != 201 {
			t.Fatalf("register %s = %d", email, code)
		}
	}
	for i := 0; i < 3; i++ {
		// Different casing is the same account.
		if code, _ := hdo(t, app, "POST", "/auth/login", `{"email":"ANA@example.com","password":"wrong-password"}`); code != 401 {
			t.Fatalf("attempt %d = %d, want 401", i, code)
		}
	}
	tc := NewTestClient(app)
	resp := tc.Post(t, "/auth/login", `{"email":"ana@example.com","password":"password123"}`)
	if resp.StatusCode() != 429 || string(resp.Header.Peek("Retry-After")) != "60" {
		t.Fatalf("locked account = %d Retry-After=%q, want 429 / 60", resp.StatusCode(), resp.Header.Peek("Retry-After"))
	}
	if code, _ := hdo(t, app, "POST", "/auth/login", `{"email":"bia@example.com","password":"password123"}`); code != 200 {
		t.Fatalf("other account = %d, want 200", code)
	}
}

// A successful login clears the failed attempts of the account.
func TestAuth_AccountRateLimit_SuccessResets(t *testing.T) {
	auth := NewAuth(AuthConfig{JWTSecret: hardeningSecret}).WithMemoryStore().
		WithRateLimit(0, 0).WithAccountRateLimit(3, time.Minute)
	app := hardeningApp(auth)
	hdo(t, app, "POST", "/auth/register", `{"email":"ana@example.com","password":"password123"}`)
	for round := 0; round < 3; round++ {
		for i := 0; i < 2; i++ {
			hdo(t, app, "POST", "/auth/login", `{"email":"ana@example.com","password":"wrong-password"}`)
		}
		if code, _ := hdo(t, app, "POST", "/auth/login", `{"email":"ana@example.com","password":"password123"}`); code != 200 {
			t.Fatalf("round %d: login = %d, want 200", round, code)
		}
	}
}

func TestKeycloakLogin_AccountRateLimit(t *testing.T) {
	tokens := &fakeKeycloakTokens{err: Err(401, "UNAUTHORIZED", "invalid credentials")}
	app := New(Config{DisableDefaults: true})
	app.Post("/auth/login", keycloakLoginHandler(tokens, newLoginThrottle(2, time.Minute)))
	tc := NewTestClient(app)
	for i := 0; i < 2; i++ {
		if resp := tc.PostJSON(t, "/auth/login", Map{"username": "ada", "password": "x"}); resp.StatusCode() != 401 {
			t.Fatalf("attempt %d = %d", i, resp.StatusCode())
		}
	}
	tokens.err = nil
	if resp := tc.PostJSON(t, "/auth/login", Map{"username": "Ada", "password": "right"}); resp.StatusCode() != 429 {
		t.Fatalf("locked account = %d, want 429", resp.StatusCode())
	}
	if resp := tc.PostJSON(t, "/auth/login", Map{"username": "bob", "password": "right"}); resp.StatusCode() != 200 {
		t.Fatalf("other account = %d, want 200", resp.StatusCode())
	}
}

// Asymmetric provider end to end: login, protected route, and the JWKS
// endpoint publishing the public key.
func TestJWTProviderWithKeys_AuthRoutesAndJWKS(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cfg := AuthConfig{}
	jwtp, err := JWTProviderWithKeys(cfg, JWTKeys{Signing: JWTSigningKey{ID: "2026-11", Key: key}})
	if err != nil {
		t.Fatal(err)
	}
	auth := NewAuth(AuthConfig{JWTSecret: hardeningSecret}).WithTokenProvider(jwtp).WithMemoryStore().WithRateLimit(0, 0)
	app := hardeningApp(auth)
	app.JWKSRoute("/.well-known/jwks.json", jwtp)

	hdo(t, app, "POST", "/auth/register", `{"email":"ana@example.com","password":"password123"}`)
	code, body := hdo(t, app, "POST", "/auth/login", `{"email":"ana@example.com","password":"password123"}`)
	access, _ := body["access_token"].(string)
	if code != 200 || strings.Count(access, ".") != 2 {
		t.Fatalf("login = %d %v", code, body)
	}
	if code, _ := hdo(t, app, "GET", "/me", "", "Authorization", "Bearer "+access); code != 200 {
		t.Fatalf("/me with an ES256 token = %d", code)
	}

	resp := NewTestClient(app).Get(t, "/.well-known/jwks.json")
	var set struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(resp.Body(), &set); err != nil || resp.StatusCode() != 200 {
		t.Fatalf("jwks = %d %s (%v)", resp.StatusCode(), resp.Body(), err)
	}
	if len(set.Keys) != 1 || set.Keys[0]["kid"] != "2026-11" || set.Keys[0]["alg"] != "ES256" {
		t.Fatalf("jwks = %s", resp.Body())
	}
}
