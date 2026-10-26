package nitro

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNew_AppliesDefaults(t *testing.T) {
	app := New(Config{Env: EnvDevelopment})
	if !app.defaultsApplied {
		t.Fatal("New should apply Defaults")
	}
	app.Get("/ok", func(c *Ctx) error {
		return c.OK(Map{"ok": true})
	})
	tc := NewTestClient(app)
	resp := tc.Get(t, "/ok")
	tc.AssertStatus(t, resp, 200)
	if len(resp.Header.Peek("X-Request-ID")) == 0 {
		t.Fatal("Defaults must set X-Request-ID")
	}
}

func TestNew_DisableDefaults(t *testing.T) {
	app := New(Config{DisableDefaults: true})
	if app.defaultsApplied {
		t.Fatal("DisableDefaults should skip Defaults")
	}
}

func TestDefaults_Idempotent(t *testing.T) {
	app := New()
	app.Defaults()
	app.Defaults()
	app.Get("/ok", func(c *Ctx) error { return c.OK(Map{"ok": true}) })
	tc := NewTestClient(app)
	tc.AssertStatus(t, tc.Get(t, "/ok"), 200)
}

func TestAPIStack(t *testing.T) {
	app := New(Config{DisableDefaults: true})
	app.UseStack(APIStack)
	app.Get("/ok", func(c *Ctx) error {
		return c.OK(Map{"ok": true})
	})
	tc := NewTestClient(app)
	tc.AssertStatus(t, tc.Get(t, "/ok"), 200)
}

func TestPublicStack(t *testing.T) {
	app := New(Config{Env: EnvDevelopment, DisableDefaults: true})
	app.UseStack(PublicStack)
	app.Get("/ok", func(c *Ctx) error {
		return c.OK(Map{"ok": true})
	})
	tc := NewTestClient(app)
	tc.AssertStatus(t, tc.Get(t, "/ok"), 200)
}

// TestPublicStack_FailsInProductionWithoutOptIn verifies V-14: booting
// with PublicStack (wildcard CORS) in production must fail fast (the app
// refuses to start) unless the operator explicitly acknowledges it.
func TestPublicStack_FailsInProductionWithoutOptIn(t *testing.T) {
	app := New(Config{Env: EnvProduction, DisableDefaults: true})
	app.UseStack(PublicStack)
	if app.Err() == nil {
		t.Fatal("expected a config error for PublicStack in production without AllowWildcardCORSInProd")
	}
	if err := app.Listen("127.0.0.1:0"); err == nil {
		t.Fatal("Listen must refuse to start")
	}
}

// TestPublicStack_AllowedInProductionWithOptIn verifies the explicit
// opt-in (Config.Security.AllowWildcardCORSInProd) lets PublicStack boot
// in production, still with a warning.
func TestPublicStack_AllowedInProductionWithOptIn(t *testing.T) {
	app := New(Config{
		Env:             EnvProduction,
		DisableDefaults: true,
		Security:        SecurityConfig{AllowWildcardCORSInProd: true},
	})
	app.UseStack(PublicStack)
	app.Get("/ok", func(c *Ctx) error {
		return c.OK(Map{"ok": true})
	})
	tc := NewTestClient(app)
	tc.AssertStatus(t, tc.Get(t, "/ok"), 200)
}

func TestResource_FailsWhenNoAuthMiddlewareInProduction(t *testing.T) {
	forceProdResourceAuthPanicForTest = true
	defer func() { forceProdResourceAuthPanicForTest = false }()

	app := New(Config{Env: EnvProduction, DisableDefaults: true})
	Resource(app, "/items", NewMemoryRepo[memItem]())
	if app.Err() == nil {
		t.Fatal("expected config error for unprotected Resource in production (V-04)")
	}
}

func TestResource_AllowedInProductionWithAllowUnauthenticatedResources(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	app := New(Config{
		Env:             EnvProduction,
		DisableDefaults: true,
		Security:        SecurityConfig{AllowUnauthenticatedResources: true},
	})
	Resource(app, "/items", NewMemoryRepo[memItem]())

	_ = w.Close()
	os.Stderr = orig
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !containsAll(out, "warning", "/items") {
		t.Fatalf("expected warning when AllowUnauthenticatedResources, got %q", out)
	}
}

func TestResource_NoWarningWhenPublicOptedIn(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	app := New(Config{Env: EnvProduction, DisableDefaults: true})
	Resource(app, "/items", NewMemoryRepo[memItem](), &ResourceConfig{Public: true})

	_ = w.Close()
	os.Stderr = orig
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if n > 0 {
		t.Fatalf("expected no warning when Public=true, got %q", string(buf[:n]))
	}
}

func TestResource_DefaultMiddlewareApplied(t *testing.T) {
	app := New(Config{Env: EnvDevelopment, DisableDefaults: true})
	called := false
	app.SetDefaultResourceMiddleware(func(c *Ctx) error {
		called = true
		return nil
	})
	Resource(app, "/items", NewMemoryRepo[memItem](), &ResourceConfig{NoCache: true})
	tc := NewTestClient(app)
	tc.AssertStatus(t, tc.Get(t, "/items"), 200)
	if !called {
		t.Fatal("expected default Resource middleware to run")
	}
}

func TestResource_DefaultMiddlewareSkippedWhenPublic(t *testing.T) {
	app := New(Config{Env: EnvDevelopment, DisableDefaults: true})
	called := false
	app.SetDefaultResourceMiddleware(func(c *Ctx) error {
		called = true
		return nil
	})
	Resource(app, "/items", NewMemoryRepo[memItem](), &ResourceConfig{Public: true, NoCache: true})
	tc := NewTestClient(app)
	tc.AssertStatus(t, tc.Get(t, "/items"), 200)
	if called {
		t.Fatal("Public Resource must skip SetDefaultResourceMiddleware")
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// TestLoginSession_FreshIDAndProtectedGroup covers vuln.md V-10: the
// documented session pattern must issue a brand-new session ID at login
// (never reusing a pre-login cookie) and SessionMiddleware must gate only
// the protected group, not /login itself.
func TestLoginSession_FreshIDAndProtectedGroup(t *testing.T) {
	store := MemorySessionStore()
	app := New(Config{DisableDefaults: true})

	app.Post("/login", func(c *Ctx) error {
		if _, err := LoginSession(c, store, "user-1", "session_id", time.Hour); err != nil {
			return err
		}
		return c.OK(Map{"status": "logged in"})
	})
	app.Post("/logout", func(c *Ctx) error {
		if err := LogoutSession(c, store, "session_id"); err != nil {
			return err
		}
		return c.NoContent()
	})
	protected := app.Group("", SessionMiddleware(store, "session_id"))
	protected.Get("/profile", func(c *Ctx) error {
		return c.OK(Map{"user_id": c.UserID()})
	})

	tc := NewTestClient(app)

	// A pre-existing (attacker-planted) cookie must not be reused as the
	// authenticated session's ID.
	preLogin := tc.WithHeader("Cookie", "session_id=attacker-fixed-id")
	login := preLogin.Post(t, "/login", "")
	tc.AssertStatus(t, login, 200)

	setCookie := string(login.Header.Peek("Set-Cookie"))
	if setCookie == "" || strings.Contains(setCookie, "attacker-fixed-id") {
		t.Fatalf("expected a freshly minted session cookie, got %q", setCookie)
	}

	// Extract the new session id from Set-Cookie: session_id=<id>; ...
	newID := setCookie
	if i := strings.Index(newID, "session_id="); i >= 0 {
		newID = newID[i+len("session_id="):]
	}
	if i := strings.IndexByte(newID, ';'); i >= 0 {
		newID = newID[:i]
	}
	if newID == "" || newID == "attacker-fixed-id" {
		t.Fatalf("expected a non-empty, non-attacker session id, got %q", newID)
	}

	// /profile requires the new session.
	authed := tc.WithHeader("Cookie", "session_id="+newID)
	prof := authed.Get(t, "/profile")
	tc.AssertStatus(t, prof, 200)
	tc.AssertBodyContains(t, prof, "user-1")

	// The stale attacker cookie must never authenticate anything.
	stale := tc.WithHeader("Cookie", "session_id=attacker-fixed-id")
	tc.AssertStatus(t, stale.Get(t, "/profile"), 401)

	// Logout invalidates the session; it can no longer be used afterward.
	tc.AssertStatus(t, authed.Post(t, "/logout", ""), 204)
	tc.AssertStatus(t, authed.Get(t, "/profile"), 401)
}

func TestAuthRoutes(t *testing.T) {
	auth := NewAuth(AuthConfig{
		JWTSecret:       "test-secret-at-least-32-characters-long",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}).WithMemoryStore()

	app := New()
	app.AuthRoutes("/auth", auth)

	tc := NewTestClient(app)
	reg := tc.Post(t, "/auth/register", `{"email":"u@x.co","password":"12345678","name":"U"}`)
	tc.AssertStatus(t, reg, 201)

	login := tc.Post(t, "/auth/login", `{"email":"u@x.co","password":"12345678"}`)
	tc.AssertStatus(t, login, 200)
}

func TestClaimsHelpers(t *testing.T) {
	auth := NewAuth(AuthConfig{
		JWTSecret:       "test-secret-at-least-32-characters-long",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}).WithMemoryStore()

	app := New()
	app.AuthRoutes("/auth", auth)
	app.Get("/me", func(c *Ctx) error {
		claims, err := c.Claims()
		if err != nil {
			return err
		}
		return c.OK(Map{"id": c.UserID(), "email": claims.Email})
	}, JWTMiddleware(auth.Provider()))

	tc := NewTestClient(app)
	reg := tc.Post(t, "/auth/register", `{"email":"c@x.co","password":"12345678"}`)
	tc.AssertStatus(t, reg, 201)

	var m map[string]any
	if err := json.Unmarshal(reg.Body(), &m); err != nil {
		t.Fatal(err)
	}
	token, _ := m["access_token"].(string)

	me := tc.GetBearer(t, "/me", token)
	tc.AssertStatus(t, me, 200)
	tc.AssertBodyContains(t, me, "c@x.co")
}
