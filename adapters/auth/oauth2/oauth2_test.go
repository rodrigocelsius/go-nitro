package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// fakeProvider serves token, user info and GitHub emails endpoints.
func fakeProvider(t *testing.T, user any, emails any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("code") != "good" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-1", "token_type": "Bearer", "verifier": r.Form.Get("code_verifier")})
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at-1" {
			http.Error(w, "nope", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(user)
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(emails)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func cfgFor(srv *httptest.Server) domain.OAuthConfig {
	return domain.OAuthConfig{
		ClientID: "id", ClientSecret: "secret", RedirectURL: "https://app/cb",
		Endpoints: domain.OAuthEndpoints{AuthURL: srv.URL + "/auth", TokenURL: srv.URL + "/token", UserInfoURL: srv.URL + "/user"},
	}
}

func TestGitHub_CallbackUsesEmailsEndpointUnderUserInfoURL(t *testing.T) {
	srv := fakeProvider(t,
		map[string]any{"id": 42, "login": "octo"},
		[]map[string]any{{"email": "second@x.co", "verified": true}, {"email": "primary@x.co", "primary": true, "verified": false}})
	p := GitHub(cfgFor(srv))
	if p.Name() != "github" || !strings.Contains(strings.Join(p.OAuthConfig().Scopes, ","), "user:email") {
		t.Fatalf("defaults: %s %v", p.Name(), p.OAuthConfig().Scopes)
	}
	u, err := p.Callback(context.Background(), "good")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != "42" || u.Name != "octo" || u.Email != "primary@x.co" || u.EmailVerified {
		t.Fatalf("user = %+v", u)
	}
	if _, err := p.Callback(context.Background(), "bad"); err == nil {
		t.Fatal("bad code must fail")
	}
}

func TestGoogle_DefaultsAndPKCE(t *testing.T) {
	srv := fakeProvider(t, map[string]any{"id": "g1", "email": "a@x.co", "verified_email": true, "name": "Ada"}, nil)
	p := Google(cfgFor(srv))
	if len(p.OAuthConfig().Scopes) != 3 {
		t.Fatalf("scopes = %v", p.OAuthConfig().Scopes)
	}
	auth, err := url.Parse(p.AuthURLPKCE("st", "verifier-0123456789-0123456789-0123456789"))
	if err != nil || auth.Query().Get("code_challenge_method") != "S256" || auth.Query().Get("state") != "st" {
		t.Fatalf("PKCE auth url: %v", auth)
	}
	if !strings.Contains(p.AuthURL("s2"), "state=s2") {
		t.Fatal("AuthURL state")
	}
	u, err := p.CallbackPKCE(context.Background(), "good", "verifier-0123456789-0123456789-0123456789")
	if err != nil || u.Email != "a@x.co" || !u.EmailVerified || u.ID != "g1" {
		t.Fatalf("user=%+v err=%v", u, err)
	}
}

func TestFetchUser_ErrorStatus(t *testing.T) {
	srv := fakeProvider(t, nil, nil)
	p := New("x", cfgFor(srv), googleUserFunc(srv.URL+"/user"))
	tok, err := p.TokenExchange(context.Background(), "good")
	if err != nil {
		t.Fatal(err)
	}
	tok.AccessToken = "wrong"
	if _, err := p.FetchUser(context.Background(), tok); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v", err)
	}
}
