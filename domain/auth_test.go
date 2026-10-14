package domain

import (
	"testing"
	"time"
)

func TestUser_HasRole(t *testing.T) {
	u := &User{Roles: []string{"admin", "user"}}
	if !u.HasRole("admin") {
		t.Fatal("expected user to have admin role")
	}
	if u.HasRole("superadmin") {
		t.Fatal("expected user not to have superadmin role")
	}
}

func TestUser_HasAnyRole(t *testing.T) {
	u := &User{Roles: []string{"editor"}}
	if !u.HasAnyRole("admin", "editor") {
		t.Fatal("expected user to have at least one of the roles")
	}
	if u.HasAnyRole("admin", "superadmin") {
		t.Fatal("expected user not to have any of the roles")
	}
}

func TestClaims_HasRole(t *testing.T) {
	c := &Claims{Roles: []string{"viewer"}}
	if !c.HasRole("viewer") {
		t.Fatal("expected claims to have viewer role")
	}
	if c.HasRole("admin") {
		t.Fatal("expected claims not to have admin role")
	}
}

func TestClaims_HasAnyRole(t *testing.T) {
	c := &Claims{Roles: []string{"viewer"}}
	if !c.HasAnyRole("admin", "viewer") {
		t.Fatal("expected claims to have at least one of the roles")
	}
	if c.HasAnyRole("admin", "editor") {
		t.Fatal("expected claims not to have any of the roles")
	}
}

func TestSession_IsExpired(t *testing.T) {
	s := &Session{ExpiresAt: time.Now().Add(-time.Hour)}
	if !s.IsExpired() {
		t.Fatal("expected session to be expired")
	}

	s2 := &Session{ExpiresAt: time.Now().Add(time.Hour)}
	if s2.IsExpired() {
		t.Fatal("expected session not to be expired")
	}
}

func TestTokenPair_Fields(t *testing.T) {
	tp := TokenPair{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresIn:    900,
		TokenType:    "Bearer",
	}
	if tp.AccessToken != "access" {
		t.Fatalf("expected access token 'access', got %q", tp.AccessToken)
	}
	if tp.TokenType != "Bearer" {
		t.Fatalf("expected token type 'Bearer', got %q", tp.TokenType)
	}
}

// Note: SessionStore and PasswordHasher interfaces are defined in auth.go.
// Their implementations will be tested in adapter packages.
