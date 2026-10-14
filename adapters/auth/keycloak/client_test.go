package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_LoginRefreshLogout(t *testing.T) {
	var gotGrant string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch {
		case strings.HasSuffix(r.URL.Path, "/token"):
			gotGrant = r.Form.Get("grant_type")
			_ = json.NewEncoder(w).Encode(TokenResponse{
				AccessToken:  "access",
				RefreshToken: "refresh",
				ExpiresIn:    300,
				TokenType:    "Bearer",
			})
		case strings.HasSuffix(r.URL.Path, "/logout"):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := Config{
		BaseURL:      srv.URL,
		Realm:        "demo",
		ClientID:     "api",
		ClientSecret: "secret",
		HTTPClient:   srv.Client(),
	}
	c := NewClient(cfg)

	tr, err := c.Login(context.Background(), "user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	if tr.AccessToken != "access" || gotGrant != "password" {
		t.Fatalf("login %#v grant=%s", tr, gotGrant)
	}

	tr, err = c.Refresh(context.Background(), "refresh")
	if err != nil {
		t.Fatal(err)
	}
	if gotGrant != "refresh_token" {
		t.Fatalf("grant=%s", gotGrant)
	}

	if err := c.Logout(context.Background(), "refresh"); err != nil {
		t.Fatal(err)
	}
}

func TestClient_RefreshInvalidGrant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "Invalid refresh token",
		})
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{
		BaseURL:      srv.URL,
		Realm:        "demo",
		ClientID:     "api",
		ClientSecret: "secret",
		HTTPClient:   srv.Client(),
	})
	_, err := c.Refresh(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := err.(interface{ StatusCode() int })
	if !ok || he.StatusCode() != 401 {
		t.Fatalf("want UnauthorizedError 401, got %T %v", err, err)
	}
}

func TestConfig_Endpoints(t *testing.T) {
	cfg := Config{BaseURL: "http://kc:8080/", Realm: "scheduler", ClientID: "api"}
	if !strings.Contains(cfg.JWKSEndpoint(), "/realms/scheduler/protocol/openid-connect/certs") {
		t.Fatal(cfg.JWKSEndpoint())
	}
	if cfg.Issuer() != "http://kc:8080/realms/scheduler" {
		t.Fatal(cfg.Issuer())
	}
}

func TestAdmin_CreateAndGetUser(t *testing.T) {
	var tokenHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/token"):
			tokenHits++
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "admin-token"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/users"):
			w.Header().Set("Location", r.URL.Path+"/uid-1")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.Contains(r.URL.RawQuery, "username="):
			_ = json.NewEncoder(w).Encode([]User{{ID: "uid-1", Username: "ana", Enabled: true}})
		default:
			t.Logf("unhandled %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	admin := NewAdmin(Config{
		BaseURL:      srv.URL,
		Realm:        "demo",
		ClientID:     "api",
		ClientSecret: "s",
		HTTPClient:   srv.Client(),
	})

	id, err := admin.CreateUser(context.Background(), User{Username: "ana", Email: "a@b.com"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "uid-1" {
		t.Fatalf("id=%s", id)
	}
	u, err := admin.GetUserByUsername(context.Background(), "ana")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != "uid-1" || tokenHits == 0 {
		t.Fatalf("user=%#v tokens=%d", u, tokenHits)
	}
}

func TestNew_RequiresConfig(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected error")
	}
}
