package fasthttp

import (
	"context"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/valyala/fasthttp"
)

// Mock TokenProvider for testing
type mockTokenProvider struct {
	claims map[string]*domain.Claims
}

func (m *mockTokenProvider) GenerateAccessToken(ctx context.Context, user *domain.User) (string, error) {
	return "", nil
}

func (m *mockTokenProvider) GenerateRefreshToken(ctx context.Context, user *domain.User) (string, error) {
	return "", nil
}

func (m *mockTokenProvider) ValidateToken(ctx context.Context, token string) (*domain.Claims, error) {
	if claims, ok := m.claims[token]; ok {
		return claims, nil
	}
	return nil, domain.ErrInvalidToken
}

func (m *mockTokenProvider) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	return nil, nil
}

func (m *mockTokenProvider) RevokeToken(ctx context.Context, token string) error {
	return nil
}

// Mock SessionStore for testing
type mockSessionStore struct {
	sessions map[string]*domain.Session
}

func (m *mockSessionStore) Create(ctx context.Context, sess *domain.Session) error {
	m.sessions[sess.ID] = sess
	return nil
}

func (m *mockSessionStore) Get(ctx context.Context, id string) (*domain.Session, error) {
	sess, ok := m.sessions[id]
	if !ok || time.Now().After(sess.ExpiresAt) {
		return nil, domain.ErrNotFound
	}
	return sess, nil
}

func (m *mockSessionStore) Update(ctx context.Context, sess *domain.Session) error {
	m.sessions[sess.ID] = sess
	return nil
}

func (m *mockSessionStore) Delete(ctx context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockSessionStore) DeleteByUserID(ctx context.Context, userID string) error {
	return nil
}

// Tests

func TestJWTMiddleware_Success(t *testing.T) {
	provider := &mockTokenProvider{
		claims: map[string]*domain.Claims{
			"valid-token": {
				UserID: "user-1",
				Email:  "test@example.com",
				Roles:  []string{"admin"},
			},
		},
	}

	app := New()
	app.Use(JWTMiddleware(provider))
	app.Get("/protected", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/protected")
	ctx.Request.Header.Set("Authorization", "Bearer valid-token")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", ctx.Response.StatusCode())
	}
	if string(ctx.Response.Body()) != "ok" {
		t.Fatalf("expected body 'ok', got %q", string(ctx.Response.Body()))
	}
}

func TestJWTMiddleware_MissingHeader(t *testing.T) {
	provider := &mockTokenProvider{claims: make(map[string]*domain.Claims)}
	app := New()
	app.Use(JWTMiddleware(provider))
	app.Get("/protected", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/protected")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 401 {
		t.Fatalf("expected 401, got %d", ctx.Response.StatusCode())
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	provider := &mockTokenProvider{claims: make(map[string]*domain.Claims)}
	app := New()
	app.Use(JWTMiddleware(provider))
	app.Get("/protected", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/protected")
	ctx.Request.Header.Set("Authorization", "Bearer invalid-token")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 401 {
		t.Fatalf("expected 401, got %d", ctx.Response.StatusCode())
	}
}

func TestSessionMiddleware_Success(t *testing.T) {
	store := &mockSessionStore{
		sessions: map[string]*domain.Session{
			"session-1": {
				ID:        "session-1",
				UserID:    "user-1",
				Data:      map[string]any{"user_id": "user-1"},
				ExpiresAt: time.Now().Add(time.Hour),
			},
		},
	}

	app := New()
	app.Use(SessionMiddleware(store, "nitro_session"))
	app.Get("/protected", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/protected")
	ctx.Request.Header.Set("Cookie", "nitro_session=session-1")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", ctx.Response.StatusCode())
	}
}

func TestSessionMiddleware_MissingSession(t *testing.T) {
	store := &mockSessionStore{sessions: make(map[string]*domain.Session)}
	app := New()
	app.Use(SessionMiddleware(store, "nitro_session"))
	app.Get("/protected", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/protected")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 401 {
		t.Fatalf("expected 401, got %d", ctx.Response.StatusCode())
	}
}

func TestRequireRole_Success(t *testing.T) {
	provider := &mockTokenProvider{
		claims: map[string]*domain.Claims{
			"admin-token": {
				UserID: "user-1",
				Email:  "admin@example.com",
				Roles:  []string{"admin"},
			},
		},
	}

	app := New()
	app.Use(JWTMiddleware(provider))
	app.Use(RequireRole("admin"))
	app.Get("/admin", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("admin ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/admin")
	ctx.Request.Header.Set("Authorization", "Bearer admin-token")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", ctx.Response.StatusCode())
	}
}

func TestRequireRoles_Bypass(t *testing.T) {
	provider := &mockTokenProvider{
		claims: map[string]*domain.Claims{
			"super-token": {
				UserID: "user-1",
				Roles:  []string{"super_admin"},
			},
		},
	}

	app := New()
	app.Use(JWTMiddleware(provider))
	app.Use(RequireRoles([]string{"admin"}, WithBypassRoles("super_admin")))
	app.Get("/admin", func(c *domain.Ctx) error {
		c.AdapterData.(*FastCtx).SetBody([]byte("ok"))
		return nil
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/admin")
	ctx.Request.Header.Set("Authorization", "Bearer super-token")
	app.Handler()(ctx)

	if ctx.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", ctx.Response.StatusCode())
	}
}
