package usecases

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestMatchPathLinear_Static(t *testing.T) {
	c := &domain.Ctx{}
	if !MatchPathLinear("/users", "/users", c) {
		t.Fatal("expected match for /users")
	}
	if MatchPathLinear("/users", "/posts", c) {
		t.Fatal("expected no match for /posts")
	}
}

func TestMatchPathLinear_Param(t *testing.T) {
	c := &domain.Ctx{}
	if !MatchPathLinear("/users/:id", "/users/123", c) {
		t.Fatal("expected match")
	}
	if c.Param("id") != "123" {
		t.Fatalf("expected param 'id' = '123', got %q", c.Param("id"))
	}
}

func TestMatchPathLinear_MultipleParams(t *testing.T) {
	c := &domain.Ctx{}
	if !MatchPathLinear("/users/:id/posts/:postId", "/users/42/posts/99", c) {
		t.Fatal("expected match")
	}
	if c.Param("id") != "42" {
		t.Fatalf("expected param 'id' = '42', got %q", c.Param("id"))
	}
	if c.Param("postId") != "99" {
		t.Fatalf("expected param 'postId' = '99', got %q", c.Param("postId"))
	}
}

func TestMatchLinear(t *testing.T) {
	handler := func(c *domain.Ctx) error { return nil }
	routes := []domain.LinearRoute{
		{Method: "GET", Path: "/users", Handler: handler},
		{Method: "POST", Path: "/users", Handler: handler},
		{Method: "GET", Path: "/users/:id", Handler: handler},
		{Method: "POST", Path: "/users/:id", Handler: handler},
	}

	c := &domain.Ctx{}
	h, ok := MatchLinear(routes, "GET", "/users", c)
	if !ok || h == nil {
		t.Fatal("expected to find GET /users")
	}

	c = &domain.Ctx{}
	h, ok = MatchLinear(routes, "POST", "/users/123", c)
	if !ok || h == nil {
		t.Fatal("expected to find POST /users/:id")
	}
	if c.Param("id") != "123" {
		t.Fatalf("expected param 'id' = '123', got %q", c.Param("id"))
	}

	c = &domain.Ctx{}
	_, ok = MatchLinear(routes, "DELETE", "/users", c)
	if ok {
		t.Fatal("expected no match for DELETE /users")
	}
}
