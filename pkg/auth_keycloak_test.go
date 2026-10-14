package nitro

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeKeycloakTokens struct {
	loginUser string
	loginPass string
	refresh   string
	err       error
}

func (f *fakeKeycloakTokens) Login(ctx context.Context, username, password string) (*KeycloakTokenResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.loginUser, f.loginPass = username, password
	return &KeycloakTokenResponse{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		ExpiresIn:    60,
	}, nil
}

func (f *fakeKeycloakTokens) Refresh(ctx context.Context, refreshToken string) (*KeycloakTokenResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.refresh = refreshToken
	return &KeycloakTokenResponse{
		AccessToken:  "access-2",
		RefreshToken: "refresh-2",
		TokenType:    "Bearer",
		ExpiresIn:    60,
	}, nil
}

func (f *fakeKeycloakTokens) Logout(ctx context.Context, refreshToken string) error {
	return f.err
}

func TestKeycloakLoginHandler(t *testing.T) {
	tokens := &fakeKeycloakTokens{}
	app := New(Config{DisableDefaults: true})
	app.Post("/auth/login", keycloakLoginHandler(tokens))

	tc := NewTestClient(app)
	resp := tc.PostJSON(t, "/auth/login", Map{"username": "ada", "password": "secret"})
	tc.AssertStatus(t, resp, 200)
	if tokens.loginUser != "ada" || tokens.loginPass != "secret" {
		t.Fatalf("login args = %q/%q", tokens.loginUser, tokens.loginPass)
	}
	var body map[string]any
	if err := json.Unmarshal(resp.Body(), &body); err != nil {
		t.Fatal(err)
	}
	if body["access_token"] != "access-1" {
		t.Fatalf("body = %#v", body)
	}
}

func TestKeycloakLoginHandler_Validation(t *testing.T) {
	app := New(Config{DisableDefaults: true})
	app.Post("/auth/login", keycloakLoginHandler(&fakeKeycloakTokens{}))
	tc := NewTestClient(app)
	resp := tc.PostJSON(t, "/auth/login", Map{"username": "ada"})
	if resp.StatusCode() < 400 {
		t.Fatalf("status = %d, want 4xx", resp.StatusCode())
	}
}

func TestKeycloakRefreshHandler(t *testing.T) {
	tokens := &fakeKeycloakTokens{}
	app := New(Config{DisableDefaults: true})
	app.Post("/auth/refresh", keycloakRefreshHandler(tokens))

	tc := NewTestClient(app)
	resp := tc.PostJSON(t, "/auth/refresh", Map{"refresh_token": "rt-old"})
	tc.AssertStatus(t, resp, 200)
	if tokens.refresh != "rt-old" {
		t.Fatalf("refresh = %q", tokens.refresh)
	}
}

func TestKeycloakLoginHandler_UpstreamError(t *testing.T) {
	tokens := &fakeKeycloakTokens{err: errors.New("bad credentials")}
	app := New(Config{DisableDefaults: true})
	app.Post("/auth/login", keycloakLoginHandler(tokens))
	tc := NewTestClient(app)
	resp := tc.PostJSON(t, "/auth/login", Map{"username": "ada", "password": "x"})
	if resp.StatusCode() < 400 {
		t.Fatalf("status = %d, want error", resp.StatusCode())
	}
}

func TestKeycloakAuthRoutes_NilIsConfigError(t *testing.T) {
	app := New(Config{DisableDefaults: true})
	app.KeycloakAuthRoutes("/auth", nil)
	if app.Err() == nil {
		t.Fatal("expected config error")
	}
}
