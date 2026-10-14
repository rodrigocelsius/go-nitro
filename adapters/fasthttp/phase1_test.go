package fasthttp

import (
	"bytes"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// =============================================================================
// Tests 1.1: Static Routes First
// =============================================================================

func TestStaticRoutesFirst_MapLookup(t *testing.T) {
	rt := NewRadixRouter()
	handler := func(_ *domain.Ctx) error { return nil }
	rt.Add("GET", "/health", handler)
	rt.Add("GET", "/users", handler)
	rt.Add("POST", "/users", handler)

	c := &domain.Ctx{}

	// Must find via O(1) map
	h, ok := rt.Lookup([]byte("GET"), []byte("/health"), c)
	if !ok || h == nil {
		t.Error("Static route /health not found via fast path")
	}

	h, ok = rt.Lookup([]byte("GET"), []byte("/users"), c)
	if !ok || h == nil {
		t.Error("Static route /users not found via fast path")
	}

	h, ok = rt.Lookup([]byte("POST"), []byte("/users"), c)
	if !ok || h == nil {
		t.Error("Static POST /users not found via fast path")
	}
}

func TestStaticRoutesFirst_DynamicFallback(t *testing.T) {
	rt := NewRadixRouter()
	handler := func(_ *domain.Ctx) error { return nil }
	rt.Add("GET", "/users/:id", handler)

	c := &domain.Ctx{}
	h, ok := rt.Lookup([]byte("GET"), []byte("/users/123"), c)
	if !ok || h == nil {
		t.Error("Dynamic route /users/:id not found")
	}
	if c.Param("id") != "123" {
		t.Errorf("Param(id) = %q, want 123", c.Param("id"))
	}
}

func TestStaticRoutesFirst_MixedRoutes(t *testing.T) {
	rt := NewRadixRouter()
	staticHandler := func(_ *domain.Ctx) error { return nil }
	dynamicHandler := func(_ *domain.Ctx) error { return nil }

	rt.Add("GET", "/health", staticHandler)
	rt.Add("GET", "/users/:id", dynamicHandler)

	c := &domain.Ctx{}

	// Static via fast path
	h1, ok1 := rt.Lookup([]byte("GET"), []byte("/health"), c)
	if !ok1 || h1 == nil {
		t.Error("Static route not found")
	}

	// Dynamic via tree
	h2, ok2 := rt.Lookup([]byte("GET"), []byte("/users/456"), c)
	if !ok2 || h2 == nil {
		t.Error("Dynamic route not found")
	}
	if c.Param("id") != "456" {
		t.Errorf("Param(id) = %q, want 456", c.Param("id"))
	}
}

func BenchmarkStaticRoute_FastPath(b *testing.B) {
	rt := NewRadixRouter()
	rt.Add("GET", "/health", dummyHandler)
	c := &domain.Ctx{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.Lookup([]byte("GET"), []byte("/health"), c)
	}
}

func BenchmarkStaticRoute_TreePath(b *testing.B) {
	// Compare: fast path vs tree for the same route
	rt := NewRadixRouter()
	rt.Add("GET", "/health", dummyHandler)
	c := &domain.Ctx{}

	b.Run("FastPath", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rt.Lookup([]byte("GET"), []byte("/health"), c)
		}
	})

	b.Run("TreePath", func(b *testing.B) {
		// Force tree use (dynamic route)
		rt2 := NewRadixRouter()
		rt2.Add("GET", "/:path", dummyHandler)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rt2.Lookup([]byte("GET"), []byte("/health"), c)
		}
	})
}

func BenchmarkStaticRoute_ManyRoutes(b *testing.B) {
	rt := NewRadixRouter()
	// Register 100 static routes
	for i := 0; i < 100; i++ {
		path := "/route" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		rt.Add("GET", path, dummyHandler)
	}
	c := &domain.Ctx{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.Lookup([]byte("GET"), []byte("/routea0"), c)
	}
}

// =============================================================================
// Tests 1.2: sync.Pool for Buffers
// =============================================================================

func TestBufferPool_Reuse(t *testing.T) {
	buf1 := GetBuffer()
	buf1.WriteString("hello")
	PutBuffer(buf1)

	buf2 := GetBuffer()
	if buf2.Len() != 0 {
		t.Errorf("Buffer should be reset, got len %d", buf2.Len())
	}
	PutBuffer(buf2)
}

func TestBufferWrite(t *testing.T) {
	data := []byte("hello world")
	result := BufferWrite(data)
	if !bytes.Equal(result, data) {
		t.Errorf("BufferWrite() = %q, want %q", result, data)
	}
}

func TestBufferWriteString(t *testing.T) {
	s := "hello world"
	result := BufferWriteString(s)
	if string(result) != s {
		t.Errorf("BufferWriteString() = %q, want %q", result, s)
	}
}

func TestBufferConcat(t *testing.T) {
	parts := [][]byte{[]byte("hello"), []byte(" "), []byte("world")}
	result := BufferConcat(parts...)
	if string(result) != "hello world" {
		t.Errorf("BufferConcat() = %q, want %q", result, "hello world")
	}
}

func TestBufferPool_Concurrent(t *testing.T) {
	// Concurrency test — must not panic
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			buf := GetBuffer()
			buf.WriteString("test")
			PutBuffer(buf)
			done <- true
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}

func BenchmarkBufferPool_WithPool(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetBuffer()
		buf.WriteString("hello world")
		PutBuffer(buf)
	}
}

func BenchmarkBufferPool_WithoutPool(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := new(bytes.Buffer)
		buf.WriteString("hello world")
		_ = buf.Bytes()
	}
}

func BenchmarkBufferWrite(b *testing.B) {
	data := []byte("hello world")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		BufferWrite(data)
	}
}

func BenchmarkBufferWriteString(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		BufferWriteString("hello world")
	}
}

func BenchmarkBufferConcat(b *testing.B) {
	parts := [][]byte{[]byte("hello"), []byte(" "), []byte("world")}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		BufferConcat(parts...)
	}
}

// =============================================================================
// Tests 1.3: Byte-level Path Comparison
// =============================================================================

func TestMatchPathBytes(t *testing.T) {
	tests := []struct {
		path []byte
		s    string
		want bool
	}{
		{[]byte("/health"), "/health", true},
		{[]byte("/health"), "/users", false},
		{[]byte("/users/123"), "/users/123", true},
		{[]byte(""), "", true},
		{[]byte("/a"), "/ab", false},
		{[]byte("/ab"), "/a", false},
	}

	for _, tt := range tests {
		got := matchPathBytes(tt.path, tt.s)
		if got != tt.want {
			t.Errorf("matchPathBytes(%q, %q) = %v, want %v", tt.path, tt.s, got, tt.want)
		}
	}
}

func TestMatchPathBytesPrefix(t *testing.T) {
	tests := []struct {
		path   []byte
		prefix string
		want   bool
	}{
		{[]byte("/users/123"), "/users/", true},
		{[]byte("/users/"), "/users/", true},
		{[]byte("/users"), "/users/", false},
		{[]byte("/health"), "/users", false},
		{[]byte(""), "", true},
	}

	for _, tt := range tests {
		got := matchPathBytesPrefix(tt.path, tt.prefix)
		if got != tt.want {
			t.Errorf("matchPathBytesPrefix(%q, %q) = %v, want %v", tt.path, tt.prefix, got, tt.want)
		}
	}
}

func TestExtractParamAfter(t *testing.T) {
	tests := []struct {
		path      []byte
		prefixLen int
		want      string
	}{
		{[]byte("/users/123"), 7, "123"},
		{[]byte("/users/"), 7, ""},
		{[]byte("/users/123/posts"), 7, "123/posts"},
	}

	for _, tt := range tests {
		got := extractParamAfter(tt.path, tt.prefixLen)
		if got != tt.want {
			t.Errorf("extractParamAfter(%q, %d) = %q, want %q", tt.path, tt.prefixLen, got, tt.want)
		}
	}
}

func TestMethodBytes(t *testing.T) {
	tests := []struct {
		m    []byte
		s    string
		want bool
	}{
		{[]byte("GET"), "GET", true},
		{[]byte("GET"), "POST", false},
		{[]byte("POST"), "POST", true},
		{[]byte(""), "", true},
	}

	for _, tt := range tests {
		got := methodBytes(tt.m, tt.s)
		if got != tt.want {
			t.Errorf("methodBytes(%q, %q) = %v, want %v", tt.m, tt.s, got, tt.want)
		}
	}
}

func BenchmarkMatchPathBytes(b *testing.B) {
	p := []byte("/health")
	s := "/health"

	b.Run("Bytes", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			matchPathBytes(p, s)
		}
	})

	b.Run("String", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = string(p) == s
		}
	})
}

func BenchmarkMatchPathBytesPrefix(b *testing.B) {
	p := []byte("/users/123")
	prefix := "/users/"

	b.Run("Bytes", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			matchPathBytesPrefix(p, prefix)
		}
	})

	b.Run("Strings.HasPrefix", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = len(p) >= len(prefix) && string(p[:len(prefix)]) == prefix
		}
	})
}

func BenchmarkMethodBytes(b *testing.B) {
	m := []byte("GET")
	s := "GET"

	b.Run("Bytes", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			methodBytes(m, s)
		}
	})

	b.Run("String", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = string(m) == s
		}
	})
}

// =============================================================================
// Tests 1.4: Param Extraction (already implemented in domain/ctx.go)
// =============================================================================

func TestParamExtraction_FixedArray(t *testing.T) {
	c := &domain.Ctx{}
	c.SetParam("id", "123")
	c.SetParam("name", "john")

	if c.Param("id") != "123" {
		t.Errorf("Param(id) = %q, want 123", c.Param("id"))
	}
	if c.Param("name") != "john" {
		t.Errorf("Param(name) = %q, want john", c.Param("name"))
	}
	if c.Param("missing") != "" {
		t.Errorf("Param(missing) = %q, want empty", c.Param("missing"))
	}
}

func TestParamExtraction_MaxParams(t *testing.T) {
	c := &domain.Ctx{}
	// Test with MaxParams (8)
	for i := 0; i < 10; i++ {
		c.SetParam(string(rune('a'+i)), "value")
	}
	// Must accept only 8
	if c.ParamCount != domain.MaxParams {
		t.Errorf("ParamCount = %d, want %d", c.ParamCount, domain.MaxParams)
	}
}

func BenchmarkParamExtraction(b *testing.B) {
	c := &domain.Ctx{}
	c.SetParam("id", "123")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Param("id")
	}
}

func BenchmarkParamExtraction_Multiple(b *testing.B) {
	c := &domain.Ctx{}
	c.SetParam("id", "123")
	c.SetParam("name", "john")
	c.SetParam("action", "view")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Param("id")
		c.Param("name")
		c.Param("action")
	}
}
