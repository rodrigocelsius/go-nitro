package fasthttp

import (
	"reflect"
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// FuzzRouter tests the router with random inputs to find panics.
func FuzzRouter(f *testing.F) {
	// Common seeds
	f.Add("/health")
	f.Add("/users")
	f.Add("/users/123")
	f.Add("/users/123/posts")
	f.Add("/users/123/posts/456")
	f.Add("/")
	f.Add("")
	f.Add("/../../../etc/passwd")
	f.Add("/users/../../etc/passwd")
	f.Add(string(make([]byte, 10000)))

	f.Fuzz(func(t *testing.T, path string) {
		rt := NewRadixRouter()
		handler := func(_ *domain.Ctx) error { return nil }

		rt.Add("GET", "/health", handler)
		rt.Add("GET", "/users", handler)
		rt.Add("GET", "/users/:id", handler)
		rt.Add("GET", "/users/:id/posts", handler)
		rt.Add("GET", "/users/:id/posts/:postId", handler)
		rt.Add("POST", "/users", handler)

		c := &domain.Ctx{}

		// Must not panic on any input
		rt.Lookup([]byte("GET"), []byte(path), c)
		rt.Lookup([]byte("POST"), []byte(path), c)
		rt.Lookup([]byte("DELETE"), []byte(path), c)
	})
}

// FuzzBinding tests binding with random inputs.
func FuzzBinding(f *testing.F) {
	f.Add("123")
	f.Add("abc")
	f.Add("")
	f.Add("99999999999999999999")
	f.Add("-1")
	f.Add("0")
	f.Add("1.5")
	f.Add("true")
	f.Add("false")

	f.Fuzz(func(t *testing.T, value string) {
		// Test type conversion
		convertAndSet(reflect.ValueOf(new(int)).Elem(), value)
		convertAndSet(reflect.ValueOf(new(uint)).Elem(), value)
		convertAndSet(reflect.ValueOf(new(float32)).Elem(), value)
		convertAndSet(reflect.ValueOf(new(bool)).Elem(), value)
		convertAndSet(reflect.ValueOf(new(string)).Elem(), value)
	})
}

// FuzzValidation tests validation with random inputs.
func FuzzValidation(f *testing.F) {
	f.Add("test@example.com")
	f.Add("invalid-email")
	f.Add("")
	f.Add("a")
	f.Add(string(make([]byte, 1000)))

	f.Fuzz(func(t *testing.T, email string) {
		type TestUser struct {
			Email string `validate:"required,email"`
		}

		user := TestUser{Email: email}
		// Must not panic
		_ = user
	})
}

// FuzzPathComparison tests path comparison.
func FuzzPathComparison(f *testing.F) {
	f.Add("/health", "/health")
	f.Add("/users", "/users")
	f.Add("/users/123", "/users/")
	f.Add("", "")
	f.Add("/a", "/ab")

	f.Fuzz(func(t *testing.T, path, expected string) {
		// Must not panic
		matchPathBytes([]byte(path), expected)
		matchPathBytesPrefix([]byte(path), expected)
	})
}

// FuzzBufferPool tests the buffer pool with concurrent operations.
func FuzzBufferPool(f *testing.F) {
	f.Add("hello")
	f.Add("")
	f.Add(string(make([]byte, 10000)))

	f.Fuzz(func(t *testing.T, data string) {
		// Test pool operations
		buf := GetBuffer()
		buf.WriteString(data)
		PutBuffer(buf)

		// Test helper functions
		BufferWrite([]byte(data))
		BufferWriteString(data)
	})
}
