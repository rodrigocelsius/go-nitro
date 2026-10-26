package fasthttp

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func dummyHandler(_ *domain.Ctx) error { return nil }

func TestRadixRouter_StaticRoute(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/users", dummyHandler)

	c := &domain.Ctx{}
	h, ok := rt.Lookup([]byte("GET"), []byte("/users"), c)
	if !ok {
		t.Fatal("Lookup failed for /users")
	}
	if h == nil {
		t.Fatal("Handler is nil")
	}
}

func TestRadixRouter_ParamRoute(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/users/:id", dummyHandler)

	c := &domain.Ctx{}
	h, ok := rt.Lookup([]byte("GET"), []byte("/users/123"), c)
	if !ok {
		t.Fatal("Lookup failed for /users/123")
	}
	if h == nil {
		t.Fatal("Handler is nil")
	}
	if c.Param("id") != "123" {
		t.Errorf("Param(id) = %q, want 123", c.Param("id"))
	}
}

func TestRadixRouter_MultipleParams(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/users/:userId/posts/:postId", dummyHandler)

	c := &domain.Ctx{}
	h, ok := rt.Lookup([]byte("GET"), []byte("/users/42/posts/99"), c)
	if !ok {
		t.Fatal("Lookup failed")
	}
	if h == nil {
		t.Fatal("Handler is nil")
	}
	if c.Param("userId") != "42" {
		t.Errorf("Param(userId) = %q, want 42", c.Param("userId"))
	}
	if c.Param("postId") != "99" {
		t.Errorf("Param(postId) = %q, want 99", c.Param("postId"))
	}
}

func TestRadixRouter_NotFound(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/users", dummyHandler)

	c := &domain.Ctx{}
	_, ok := rt.Lookup([]byte("GET"), []byte("/posts"), c)
	if ok {
		t.Error("Lookup should fail for unmatched route")
	}
}

func TestRadixRouter_MethodMismatch(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/users", dummyHandler)

	c := &domain.Ctx{}
	_, ok := rt.Lookup([]byte("POST"), []byte("/users"), c)
	if ok {
		t.Error("Lookup should fail for wrong method")
	}
}

func TestRadixRouter_SamePathDifferentMethods(t *testing.T) {
	rt := NewRadixRouter()
	getCalled := false
	postCalled := false

	rt.Add("GET", "/data", func(_ *domain.Ctx) error {
		getCalled = true
		return nil
	})
	rt.Add("POST", "/data", func(_ *domain.Ctx) error {
		postCalled = true
		return nil
	})

	c := &domain.Ctx{}

	h, _ := rt.Lookup([]byte("GET"), []byte("/data"), c)
	h(c)
	if !getCalled {
		t.Error("GET handler not called")
	}

	h, _ = rt.Lookup([]byte("POST"), []byte("/data"), c)
	h(c)
	if !postCalled {
		t.Error("POST handler not called")
	}
}

func TestRadixRouter_Routes(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/users", dummyHandler)
	rt.Add("POST", "/users", dummyHandler)
	rt.Add("GET", "/users/:id", dummyHandler)

	routes := rt.Routes()
	if len(routes) != 3 {
		t.Errorf("Routes() returned %d routes, want 3", len(routes))
	}

	found := map[string]bool{}
	for _, r := range routes {
		key := r.Method + " " + r.Path
		found[key] = true
	}

	expected := []string{"GET /users", "POST /users", "GET /users/:id"}
	for _, e := range expected {
		if !found[e] {
			t.Errorf("Missing route: %s", e)
		}
	}
}

func TestRadixRouter_NestedStatic(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/api/v1/users", dummyHandler)
	rt.Add("GET", "/api/v1/posts", dummyHandler)

	c := &domain.Ctx{}
	h, ok := rt.Lookup([]byte("GET"), []byte("/api/v1/users"), c)
	if !ok || h == nil {
		t.Error("Lookup failed for /api/v1/users")
	}

	h, ok = rt.Lookup([]byte("GET"), []byte("/api/v1/posts"), c)
	if !ok || h == nil {
		t.Error("Lookup failed for /api/v1/posts")
	}
}

func TestRadixRouter_RootPath(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/", dummyHandler)

	c := &domain.Ctx{}
	h, ok := rt.Lookup([]byte("GET"), []byte("/"), c)
	if !ok {
		t.Error("Lookup failed for /")
	}
	if h == nil {
		t.Error("Handler is nil for /")
	}
}

func TestSplitSegments(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/users", []string{"users"}},
		{"/users/123", []string{"users", "123"}},
		{"/api/v1/users", []string{"api", "v1", "users"}},
		{"/", nil},
		{"", nil},
	}

	for _, tt := range tests {
		got := splitSegments(tt.path)
		if len(got) != len(tt.want) {
			t.Errorf("splitSegments(%q) = %v, want %v", tt.path, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitSegments(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
			}
		}
	}
}

func BenchmarkRadixRouter_LookupStatic(b *testing.B) {
	rt := NewRadixRouter()
	rt.Add("GET", "/users", dummyHandler)

	c := &domain.Ctx{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rt.Lookup([]byte("GET"), []byte("/users"), c)
	}
}

func BenchmarkRadixRouter_LookupParam(b *testing.B) {
	rt := NewRadixRouter()
	rt.Add("GET", "/users/:id", dummyHandler)

	c := &domain.Ctx{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rt.Lookup([]byte("GET"), []byte("/users/123"), c)
	}
}

func TestRadixRouter_SortChildrenByHits(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/a", dummyHandler)
	rt.Add("GET", "/b", dummyHandler)
	rt.Add("GET", "/c", dummyHandler)

	c := &domain.Ctx{}

	// Access /b 10 times
	for i := 0; i < 10; i++ {
		rt.Lookup([]byte("GET"), []byte("/b"), c)
	}
	// Access /c 5 times
	for i := 0; i < 5; i++ {
		rt.Lookup([]byte("GET"), []byte("/c"), c)
	}

	// Check hits before sort
	methodNode := rt.methodNode("GET")

	rt.SortChildrenByHits()

	// After sort, /b should be first (10 hits), then /c (5 hits), then /a (0 hits)
	if methodNode.children[0].hits < methodNode.children[1].hits {
		t.Error("children not sorted by hits")
	}
}

func TestRadixRouter_FlattenTree(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/users", dummyHandler)
	rt.Add("GET", "/users/:id", dummyHandler)
	rt.Add("POST", "/users", dummyHandler)

	flat := rt.FlattenTree()
	if len(flat) == 0 {
		t.Error("FlattenTree returned empty")
	}

	// Should have at least: root + GET + /users + /users/:id + POST
	if len(flat) < 4 {
		t.Errorf("FlattenTree returned %d nodes, want >= 4", len(flat))
	}
}

func TestRadixRouter_HitsIncrement(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/users/:id", dummyHandler) // Dynamic route

	c := &domain.Ctx{}
	for i := 0; i < 100; i++ {
		rt.Lookup([]byte("GET"), []byte("/users/123"), c)
	}

	methodNode := rt.methodNode("GET")
	if methodNode.hits != 100 {
		t.Errorf("hits = %d, want 100", methodNode.hits)
	}
}

func TestRadixRouter_CatchAllRoute(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/static/*filepath", dummyHandler)

	c := &domain.Ctx{}
	h, ok := rt.Lookup([]byte("GET"), []byte("/static/js/app.js"), c)
	if !ok {
		t.Fatal("Lookup failed for /static/js/app.js")
	}
	if h == nil {
		t.Fatal("Handler is nil")
	}
	if c.Param("filepath") != "js/app.js" {
		t.Errorf("filepath = %q, want js/app.js", c.Param("filepath"))
	}
}

func TestRadixRouter_CatchAllRoot(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/*filepath", dummyHandler)

	c := &domain.Ctx{}
	h, ok := rt.Lookup([]byte("GET"), []byte("/anything/here"), c)
	if !ok {
		t.Fatal("Lookup failed for /anything/here")
	}
	if c.Param("filepath") != "anything/here" {
		t.Errorf("filepath = %q, want anything/here", c.Param("filepath"))
	}

	// Also test root
	h, ok = rt.Lookup([]byte("GET"), []byte("/"), c)
	if !ok {
		t.Fatal("Lookup failed for /")
	}
	_ = h
}

func TestRadixRouter_CatchAllWithStaticPrefix(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/api/users", dummyHandler)
	rt.Add("GET", "/static/*filepath", dummyHandler)

	// Static route should still work
	c := &domain.Ctx{}
	_, ok := rt.Lookup([]byte("GET"), []byte("/api/users"), c)
	if !ok {
		t.Fatal("Static route /api/users not found")
	}

	// Catch-all route should work
	c = &domain.Ctx{}
	_, ok = rt.Lookup([]byte("GET"), []byte("/static/css/style.css"), c)
	if !ok {
		t.Fatal("Catch-all route /static/css/style.css not found")
	}
	if c.Param("filepath") != "css/style.css" {
		t.Errorf("filepath = %q, want css/style.css", c.Param("filepath"))
	}
}

func TestRadixRouter_CatchAllParamConflict(t *testing.T) {
	rt := NewRadixRouter()
	rt.Add("GET", "/users/:id", dummyHandler)
	rt.Add("GET", "/*filepath", dummyHandler)

	// Param route should take precedence
	c := &domain.Ctx{}
	h, ok := rt.Lookup([]byte("GET"), []byte("/users/123"), c)
	if !ok {
		t.Fatal("Param route /users/123 not found")
	}
	if c.Param("id") != "123" {
		t.Errorf("id = %q, want 123", c.Param("id"))
	}
	_ = h

	// Catch-all should match other paths
	c = &domain.Ctx{}
	h, ok = rt.Lookup([]byte("GET"), []byte("/something/else"), c)
	if !ok {
		t.Fatal("Catch-all route /something/else not found")
	}
	if c.Param("filepath") != "something/else" {
		t.Errorf("filepath = %q, want something/else", c.Param("filepath"))
	}
	_ = h
}
