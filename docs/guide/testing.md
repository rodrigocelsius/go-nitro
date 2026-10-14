# Testing

**Language:** [English](testing.md) · [Português](../pt/guide/testing.md)

Nitro provides built-in support for testing HTTP handlers **without starting a real server**. This guide covers all testing patterns.

## Quick Start

```go
func TestGetUser(t *testing.T) {
    tc := nitro.NewTestClient(BuildApp())
    resp := tc.Get(t, "/users/42")
    tc.AssertStatus(t, resp, 200)
    tc.AssertBodyContains(t, resp, "user:42")
}
```

### SQL without Docker

Use in-memory SQLite via the SQL adapter (Go 1.25+):

```go
db, err := sqladapter.OpenTest(ctx)
if err != nil {
    t.Fatal(err)
}
defer db.Close()

// Apply goose files when present (from module root).
// Generated handler tests use ../../migrations because `go test ./internal/handler`
// runs with that package as the working directory.
db, err = sqladapter.OpenTestMigrate(ctx, filepath.Join("..", "..", "migrations"))
store := sqladapter.StoreOf[Product](db, "products")
```

`make:scaffold` / `make:test` emit a `*_store_test.go` that uses this pattern when SQL is installed.

`OpenTestFile(ctx, filepath.Join(t.TempDir(), "app.db"))` keeps a file-backed DB for debugging.

### Without TestClient (low-level API)

```go
package myapp_test

import (
    "testing"
    "bytes"
    "encoding/json"

    nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func BuildApp() *nitro.App {
    app := nitro.New()
    app.Get("/users/:id", func(c *nitro.Ctx) error {
        return c.Text("user:" + c.Param("id"))
    })
    return app
}

func TestGetUserRaw(t *testing.T) {
    app := BuildApp()
    resp, err := app.Test("GET", "/users/42", nil)
    if err != nil {
        t.Fatal(err)
    }
    if resp.StatusCode() != 200 {
        t.Fatalf("expected 200, got %d", resp.StatusCode())
    }
    if string(resp.Body()) != "user:42" {
        t.Fatalf("expected 'user:42', got %q", string(resp.Body()))
    }
}
```

## The `app.Test()` Method

The primary testing method executes a request **in-process** without opening a TCP port:

```go
func (a *App) Test(method, path string, body io.Reader) (*fasthttp.Response, error)
```

- **method**: HTTP method (`"GET"`, `"POST"`, `"PUT"`, `"DELETE"`, etc.)
- **path**: Request path (including query string if needed)
- **body**: Request body (`nil` for GET/DELETE)
- **Returns**: A `*fasthttp.Response` with `.StatusCode()`, `.Body()`, `.Header()`, etc.

### POST with JSON Body

```go
func TestCreateUser(t *testing.T) {
    app := BuildApp()
    body := `{"name":"Joao","email":"joao@test.com"}`
    resp, err := app.Test("POST", "/users", bytes.NewBufferString(body))
    if err != nil {
        t.Fatal(err)
    }
    if resp.StatusCode() != 201 {
        t.Fatalf("expected 201, got %d", resp.StatusCode())
    }

    var user struct {
        ID    int    `json:"id"`
        Name  string `json:"name"`
    }
    json.Unmarshal(resp.Body(), &user)
    if user.Name != "Joao" {
        t.Errorf("expected name 'Joao', got %q", user.Name)
    }
}
```

### Query Parameters

```go
func TestSearchUsers(t *testing.T) {
    app := BuildApp()
    resp, err := app.Test("GET", "/users?search=john&page=2", nil)
    // ...
}
```

## Low-Level Testing with `app.Handler()`

For more control, you can use `app.Handler()` which returns the underlying fasthttp handler function:

```go
func TestWithRawContext(t *testing.T) {
    app := BuildApp()
    handler := app.Handler()

    ctx := &fasthttp.RequestCtx{}
    ctx.Request.Header.SetMethod("GET")
    ctx.Request.SetRequestURI("/users/42")

    handler(ctx)

    // Assert directly on the RequestCtx
    if ctx.Response.StatusCode() != 200 {
        t.Fatalf("expected 200, got %d", ctx.Response.StatusCode())
    }
    body := string(ctx.Response.Body())
    if body != "user:42" {
        t.Fatalf("expected 'user:42', got %q", body)
    }
}
```

This pattern is useful when you need to inspect headers, cookies, or other response details not exposed by `app.Test()`.

## Testing Middleware

```go
func TestAuthMiddleware(t *testing.T) {
    app := nitro.New()
    app.Use(nitro.JWTMiddleware(provider))

    app.Get("/protected", func(c *nitro.Ctx) error {
        return c.SendString("secret")
    })

    // Request without token → 401
    resp, _ := app.Test("GET", "/protected", nil)
    if resp.StatusCode() != 401 {
        t.Fatalf("expected 401, got %d", resp.StatusCode())
    }

    // Request with valid token → 200
    resp, _ = app.Test("GET", "/protected?token=valid_jwt", nil)
    if resp.StatusCode() != 200 {
        t.Fatalf("expected 200, got %d", resp.StatusCode())
    }
}
```

## Testing Route Groups

```go
func TestAPIRoutes(t *testing.T) {
    app := nitro.New()
    api := app.Group("/api/v1")
    api.Get("/users", listUsers)
    api.Post("/users", createUser)
    api.Get("/users/:id", getUser)

    // Test each route
    cases := []struct {
        method string
        path   string
        want   int
    }{
        {"GET", "/api/v1/users", 200},
        {"POST", "/api/v1/users", 201},
        {"GET", "/api/v1/users/1", 200},
    }

    for _, tc := range cases {
        t.Run(tc.method+tc.path, func(t *testing.T) {
            resp, err := app.Test(tc.method, tc.path, nil)
            if err != nil {
                t.Fatal(err)
            }
            if resp.StatusCode() != tc.want {
                t.Errorf("got %d, want %d", resp.StatusCode(), tc.want)
            }
        })
    }
}
```

## Table-Driven Tests

Nitro tests follow the standard Go table-driven pattern:

```go
func TestPluralize(t *testing.T) {
    tests := []struct {
        input string
        want  string
    }{
        {"User", "users"},
        {"OrderItem", "order_items"},
        {"Category", "categories"},
        {"Status", "statuses"},
    }
    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            got := Pluralize(tt.input)
            if got != tt.want {
                t.Errorf("Pluralize(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

## Inspecting Registered Routes

You can programmatically verify that routes are registered correctly:

```go
func TestRoutesRegistered(t *testing.T) {
    app := BuildApp()
    routes := app.Routes()

    if len(routes) != 3 {
        t.Fatalf("expected 3 routes, got %d", len(routes))
    }

    // Check specific route exists
    found := false
    for _, r := range routes {
        if r.Path == "/users/:id" && r.Method == "GET" {
            found = true
        }
    }
    if !found {
        t.Error("route GET /users/:id not found")
    }
}
```

## Testing Error Responses

```go
func TestNotFound(t *testing.T) {
    app := nitro.New()
    app.Get("/users/:id", func(c *nitro.Ctx) error {
        return nitro.NotFound("user not found")
    })

    resp, _ := app.Test("GET", "/users/999", nil)
    if resp.StatusCode() != 404 {
        t.Fatalf("expected 404, got %d", resp.StatusCode())
    }

    var errBody struct {
        Error string `json:"error"`
    }
    json.Unmarshal(resp.Body(), &errBody)
    if errBody.Error != "user not found" {
        t.Errorf("expected error 'user not found', got %q", errBody.Error)
    }
}
```

## Integration Tests with `app.Test()`

For end-to-end scenarios, chain multiple requests:

```go
func TestCRUDFlow(t *testing.T) {
    app := BuildApp()

    // Create
    createBody := `{"name":"Alice"}`
    resp, _ := app.Test("POST", "/users", bytes.NewBufferString(createBody))
    if resp.StatusCode() != 201 {
        t.Fatalf("create: expected 201, got %d", resp.StatusCode())
    }

    var created User
    json.Unmarshal(resp.Body(), &created)

    // Read
    resp, _ = app.Test("GET", fmt.Sprintf("/users/%d", created.ID), nil)
    if resp.StatusCode() != 200 {
        t.Fatalf("read: expected 200, got %d", resp.StatusCode())
    }

    // Update
    updateBody := `{"name":"Alice Updated"}`
    resp, _ = app.Test("PATCH", fmt.Sprintf("/users/%d", created.ID), bytes.NewBufferString(updateBody))
    if resp.StatusCode() != 200 {
        t.Fatalf("update: expected 200, got %d", resp.StatusCode())
    }

    // Delete
    resp, _ = app.Test("DELETE", fmt.Sprintf("/users/%d", created.ID), nil)
    if resp.StatusCode() != 204 {
        t.Fatalf("delete: expected 204, got %d", resp.StatusCode())
    }
}
```

## Benchmarks

Nitro supports Go's built-in benchmarking. Always call `b.ReportAllocs()`:

```go
func BenchmarkGetUser(b *testing.B) {
    app := BuildApp()
    b.ReportAllocs()
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        _, _ = app.Test("GET", "/users/42", nil)
    }
}
```

Run with:

```bash
go test -bench=. -benchmem ./...
```

## Best Practices

1. **Use `app.Test()`** for most tests — it's fast and doesn't require a real server.
2. **Use `app.Handler()`** when you need low-level access to the `fasthttp.RequestCtx`.
3. **Build apps without `Listen()`** — create a `BuildApp()` factory function that returns `*nitro.App` without starting the server.
4. **Table-driven tests** are the standard pattern for testing multiple inputs.
5. **Hand-rolled mocks** — the project uses no mock framework. Create simple structs implementing the needed interfaces.
6. **No assertion library** — use `t.Fatal`/`t.Fatalf` for critical failures and `t.Error`/`t.Errorf` for non-critical ones.
