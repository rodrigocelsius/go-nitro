# Testes

**Idioma:** [Português](testing.md) · [English](../../guide/testing.md)

O Nitro oferece suporte nativo para testar handlers HTTP **sem subir um servidor de verdade**. Este guia cobre todos os padrões de teste.

## Início rápido

```go
func TestGetUser(t *testing.T) {
    tc := nitro.NewTestClient(BuildApp())
    resp := tc.Get(t, "/users/42")
    tc.AssertStatus(t, resp, 200)
    tc.AssertBodyContains(t, resp, "user:42")
}
```

### SQL sem Docker

SQLite em memória via o adapter SQL (Go 1.25+):

```go
db, err := sqladapter.OpenTest(ctx)
if err != nil {
    t.Fatal(err)
}
defer db.Close()

// Aplique goose quando existir (raiz do módulo).
// Testes gerados usam ../../migrations porque `go test ./internal/handler`
// usa esse pacote como cwd.
db, err = sqladapter.OpenTestMigrate(ctx, filepath.Join("..", "..", "migrations"))
store := sqladapter.StoreOf[Product](db, "products")
```

`make:scaffold` / `make:test` geram `*_store_test.go` com esse padrão quando SQL está instalado.

`OpenTestFile(ctx, filepath.Join(t.TempDir(), "app.db"))` para DB em arquivo.

### Sem TestClient (API de baixo nível)

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

## O método `app.Test()`

O método principal de teste executa uma requisição **in-process**, sem abrir porta TCP:

```go
func (a *App) Test(method, path string, body io.Reader) (*fasthttp.Response, error)
```

- **method**: método HTTP (`"GET"`, `"POST"`, `"PUT"`, `"DELETE"`, etc.)
- **path**: path da requisição (incluindo query string se necessário)
- **body**: body da requisição (`nil` para GET/DELETE)
- **Returns**: um `*fasthttp.Response` com `.StatusCode()`, `.Body()`, `.Header()`, etc.

### POST com body JSON

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

## Teste de baixo nível com `app.Handler()`

Para mais controle, use `app.Handler()`, que devolve a função handler do fasthttp:

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

Esse padrão é útil quando você precisa inspecionar headers, cookies ou outros detalhes da resposta que `app.Test()` não expõe.

## Testando middleware

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

## Testando grupos de rotas

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

## Table-driven tests

Os testes do Nitro seguem o padrão table-driven do Go:

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

## Inspecionando rotas registradas

Você pode verificar programaticamente se as rotas foram registradas corretamente:

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

## Testando respostas de erro

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

## Testes de integração com `app.Test()`

Para cenários ponta a ponta, encadeie várias requisições:

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

O Nitro usa o benchmarking nativo do Go. Sempre chame `b.ReportAllocs()`:

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

Execute com:

```bash
go test -bench=. -benchmem ./...
```

## Boas práticas

1. **Use `app.Test()`** na maioria dos testes — é rápido e não exige um servidor real.
2. **Use `app.Handler()`** quando precisar de acesso de baixo nível ao `fasthttp.RequestCtx`.
3. **Monte apps sem `Listen()`** — crie uma factory `BuildApp()` que devolve `*nitro.App` sem iniciar o servidor.
4. **Table-driven tests** são o padrão para testar várias entradas.
5. **Mocks feitos à mão** — o projeto não usa framework de mock. Crie structs simples que implementem as interfaces necessárias.
6. **Sem biblioteca de assertions** — use `t.Fatal`/`t.Fatalf` para falhas críticas e `t.Error`/`t.Errorf` para as demais.
