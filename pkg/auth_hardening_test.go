package nitro

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

const hardeningSecret = "0123456789abcdef0123456789abcdef-hardening"

func hardeningApp(auth *Auth) *App {
	app := New(Config{DisableDefaults: true})
	app.AuthRoutes("/auth", auth)
	app.Group("", JWTMiddleware(auth.Provider())).
		Get("/me", func(c *Ctx) error { return c.OK(Map{"id": c.UserID()}) })
	return app
}

func hdo(t *testing.T, app *App, method, path, body string, headers ...string) (int, map[string]any) {
	t.Helper()
	tc := NewTestClient(app)
	for i := 0; i+1 < len(headers); i += 2 {
		tc = tc.WithHeader(headers[i], headers[i+1])
	}
	var code int
	var raw []byte
	switch method {
	case "POST":
		r := tc.Post(t, path, body)
		code, raw = r.StatusCode(), r.Body()
	default:
		r := tc.Get(t, path)
		code, raw = r.StatusCode(), r.Body()
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return code, out
}

// E-mail identity is case-insensitive: no duplicate accounts by casing, and
// login works with any casing.
func TestAuth_EmailCaseInsensitive(t *testing.T) {
	auth := NewAuth(AuthConfig{JWTSecret: hardeningSecret}).WithMemoryStore().WithRateLimit(0, 0)
	app := hardeningApp(auth)
	if code, _ := hdo(t, app, "POST", "/auth/register", `{"email":"Ana@Example.com","password":"password123"}`); code != 201 {
		t.Fatalf("register = %d", code)
	}
	if code, _ := hdo(t, app, "POST", "/auth/register", `{"email":"ana@example.com","password":"password123"}`); code != 409 {
		t.Fatalf("same e-mail with other casing must conflict, got %d", code)
	}
	if code, _ := hdo(t, app, "POST", "/auth/login", `{"email":"ANA@EXAMPLE.COM","password":"password123"}`); code != 200 {
		t.Fatalf("login with other casing = %d", code)
	}
}

// bcrypt ignores bytes after 72: such passwords are rejected up front.
func TestAuth_PasswordMaxLength(t *testing.T) {
	auth := NewAuth(AuthConfig{JWTSecret: hardeningSecret}).WithMemoryStore().WithRateLimit(0, 0)
	app := hardeningApp(auth)
	long := strings.Repeat("a", 73)
	if code, _ := hdo(t, app, "POST", "/auth/register", `{"email":"b@example.com","password":"`+long+`"}`); code != 422 {
		t.Fatalf("73-byte password = %d, want 422", code)
	}
}

type failingCreator struct{ *MemoryUserStore }

func (failingCreator) Create(context.Context, *User) error {
	return errors.New("pq: connection refused to 10.0.0.5:5432 password=hunter2")
}

// A store failure is a 500 that does not echo internals (and is not
// mislabelled as "email exists").
func TestAuth_RegisterStoreErrorNotLeaked(t *testing.T) {
	store := NewMemoryUserStore()
	auth := NewAuth(AuthConfig{JWTSecret: hardeningSecret}).WithStore(store, failingCreator{store}).WithRateLimit(0, 0)
	app := hardeningApp(auth)
	code, body := hdo(t, app, "POST", "/auth/register", `{"email":"c@example.com","password":"password123"}`)
	if code != 500 {
		t.Fatalf("store failure = %d, want 500", code)
	}
	raw, _ := json.Marshal(body)
	if strings.Contains(string(raw), "hunter2") || strings.Contains(string(raw), "10.0.0.5") {
		t.Fatalf("internal error leaked: %s", raw)
	}
}

// Logout revokes both the refresh token and the bearer access token.
func TestAuth_LogoutRevokesTokens(t *testing.T) {
	auth := NewAuth(AuthConfig{JWTSecret: hardeningSecret}).WithMemoryStore().WithRateLimit(0, 0)
	app := hardeningApp(auth)
	_, reg := hdo(t, app, "POST", "/auth/register", `{"email":"d@example.com","password":"password123"}`)
	access, _ := reg["access_token"].(string)
	refresh, _ := reg["refresh_token"].(string)
	if code, _ := hdo(t, app, "GET", "/me", "", "Authorization", "Bearer "+access); code != 200 {
		t.Fatalf("access before logout = %d", code)
	}
	if code, _ := hdo(t, app, "POST", "/auth/logout", `{"refresh_token":"`+refresh+`"}`, "Authorization", "Bearer "+access); code != 204 {
		t.Fatalf("logout = %d", code)
	}
	if code, _ := hdo(t, app, "GET", "/me", "", "Authorization", "Bearer "+access); code != 401 {
		t.Fatalf("access token after logout = %d, want 401", code)
	}
	if code, _ := hdo(t, app, "POST", "/auth/refresh", `{"refresh_token":"`+refresh+`"}`); code != 401 {
		t.Fatalf("refresh token after logout = %d, want 401", code)
	}
}

// AuthRoutes are throttled per client IP by default.
func TestAuth_RateLimitedByDefault(t *testing.T) {
	auth := NewAuth(AuthConfig{JWTSecret: hardeningSecret}).WithMemoryStore().WithRateLimit(3, time.Minute)
	app := hardeningApp(auth)
	codes := []int{}
	for i := 0; i < 4; i++ {
		code, _ := hdo(t, app, "POST", "/auth/login", `{"email":"x@example.com","password":"wrong-password"}`)
		codes = append(codes, code)
	}
	if codes[3] != 429 {
		t.Fatalf("4th attempt over a limit of 3 = %v, want 429", codes)
	}
	if NewAuth(AuthConfig{JWTSecret: hardeningSecret}).rateMax != DefaultAuthRateLimit {
		t.Fatal("NewAuth must enable the default rate limit")
	}
}

// Concurrent registrations of one e-mail: exactly one account is created.
func TestMemoryUserStore_ConcurrentSameEmail(t *testing.T) {
	store := NewMemoryUserStore()
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := store.Create(context.Background(), &User{Email: "Race@Example.com"}); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 || store.Len() != 1 {
		t.Fatalf("created %d accounts (store len %d), want exactly 1", ok, store.Len())
	}
}
