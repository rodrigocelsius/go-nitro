package main

import (
	"encoding/json"
	"testing"
	"time"

	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func buildApp() *nitro.App {
	auth := nitro.NewAuth(nitro.AuthConfig{
		JWTSecret:       "test-secret-at-least-32-characters-long",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}).WithMemoryStore()

	app := nitro.New(nitro.Config{Env: nitro.EnvDevelopment})
	app.AuthRoutes("/auth", auth)
	protected := app.Group("", nitro.JWTMiddleware(auth.Provider()))
	protected.Get("/profile", func(c *nitro.Ctx) error {
		claims, err := c.Claims()
		if err != nil {
			return err
		}
		return c.OK(nitro.Map{"user_id": c.UserID(), "email": claims.Email})
	})
	return app
}

func TestAuth_RegisterLoginProfile(t *testing.T) {
	app := buildApp()
	tc := nitro.NewTestClient(app)

	reg := tc.Post(t, "/auth/register", `{"email":"a@b.co","password":"12345678","name":"Ada"}`)
	tc.AssertStatus(t, reg, 201)
	tc.AssertBodyContains(t, reg, "access_token")

	login := tc.Post(t, "/auth/login", `{"email":"a@b.co","password":"12345678"}`)
	tc.AssertStatus(t, login, 200)
	token := extractToken(t, login.Body())

	unauth := tc.Get(t, "/profile")
	if unauth.StatusCode() == 200 {
		t.Fatal("expected unauthorized without token")
	}

	profile := tc.GetBearer(t, "/profile", token)
	tc.AssertStatus(t, profile, 200)
	tc.AssertBodyContains(t, profile, "a@b.co")
	tc.AssertBodyContains(t, profile, "user_id")
}

func TestAuth_Refresh(t *testing.T) {
	app := buildApp()
	tc := nitro.NewTestClient(app)

	reg := tc.Post(t, "/auth/register", `{"email":"r@b.co","password":"12345678"}`)
	tc.AssertStatus(t, reg, 201)
	refresh := extractRefresh(t, reg.Body())

	resp := tc.Post(t, "/auth/refresh", `{"refresh_token":"`+refresh+`"}`)
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, "access_token")
}

func TestAuth_Validation(t *testing.T) {
	app := buildApp()
	tc := nitro.NewTestClient(app)
	tc.AssertStatus(t, tc.Post(t, "/auth/register", `{}`), 422)
}

func extractToken(t *testing.T, body []byte) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	tok, _ := m["access_token"].(string)
	if tok == "" {
		t.Fatalf("missing token: %s", body)
	}
	return tok
}

func extractRefresh(t *testing.T, body []byte) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	tok, _ := m["refresh_token"].(string)
	if tok == "" {
		t.Fatalf("missing refresh: %s", body)
	}
	return tok
}
