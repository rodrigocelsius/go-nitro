# Request — parâmetros e body

**Idioma:** [Português](request.md) · [English](../../guide/request.md)

Como ler path, query, headers, cookies, form e JSON no `*nitro.Ctx`.

## Path

```go
app.Get("/users/:id", func(c *nitro.Ctx) error {
    id := c.Param("id")           // "" se ausente
    id = c.Param("id", "0")       // default (Fiber-like)
    return c.OK(nitro.Map{"id": id})
})

app.Get("/items/:id", func(c *nitro.Ctx) error {
    id := c.ParamInt("id", 0)
    id64 := c.ParamInt64("id", 0) // default; nunca retorna error
    price := c.ParamFloat("price", 0)
    return c.OK(nitro.Map{"id": id, "id64": id64, "price": price})
})

// Obrigatório (BadRequest se inválido) — preferido em handlers DB:
id, err := c.Int64Param("id")
if err != nil {
    return err
}
```

### `ParamInt64` vs `Int64Param`

| Método | Assinatura | Quando usar |
|--------|------------|-------------|
| `c.ParamInt64("id", 0)` | `int64` | default seguro; sem erro |
| `c.Int64Param("id")` | `(int64, error)` | ID obrigatório em CRUD/Postgres |

Não confunda com `c.Int` / `c.IntE` (query string).

## Query

```go
q := c.Query("q")                 // "" se ausente
lang := c.Query("lang", "en")     // default
page := c.Int("page", 1)
perPage := c.Int("per_page", 20)
min := c.Float("min_price", 0)
active := c.Bool("active")        // true se "true"|"1"|"yes"
tags := c.QuerySlice("tag")       // ?tag=a&tag=b
field, desc := c.Sort("created_at") // ?sort=-created_at

page, err := c.IntE("page")       // erro se ausente/inválido
```

## Headers e cookies

```go
token := c.Header("Authorization")
ver := c.Header("X-API-Version", "1")

sid := c.Cookie("session_id")
c.SetCookie("session_id", token, 86400, "/", "", true, true)
c.DeleteCookie("session_id")
```

## Body (Content-Type unificado)

**Hierarquia:**

1. `nitro.Parse[T](c)` — preferido (tipado + validate → 400/422; JSON **ou** form)
2. `c.BodyParser(&v)` — bind por Content-Type + validate
3. `c.BodyJSON(&v)` — só JSON + validate
4. `c.BindJSON(&v)` — **deprecated** (alias de BodyJSON)
5. `c.Bind(&v)` — decode sem validate (JSON / form / query conforme CT)
6. `c.BindForm(&v)` — só form (`form:` / `json:` / nome do campo)
7. `c.BindMap()` — decode em `map[string]any` (sem validate)

```go
user, err := nitro.Parse[User](c)
if err != nil {
    return err
}
return c.Created(user)
```

`Parse` / `BodyParser` / `Bind` detectam `Content-Type`:

| Content-Type | Comportamento |
|--------------|---------------|
| `application/json` | JSON |
| `application/x-www-form-urlencoded` / `multipart/form-data` | form (`form:"…"`) |
| vazio + body JSON | JSON |
| vazio / sem body | query (`query:"…"`) |

Defaults no JSON: tag `default:"true"` ou `validate:"default=true"` — só se a chave **não** veio no body.  
Detalhes: [Validation](validation.md).

## Form / upload

```go
name := c.FormValue("name")
file, err := c.FormFile("avatar")

// Struct tipado (também via Parse/BodyParser se CT for form):
var in struct {
    Name string `form:"name"`
    Age  int    `form:"age"`
}
_ = c.BindForm(&in)
```

## Resumo

| Método | Tipo | Exemplo |
|--------|------|---------|
| `Param` / `ParamInt` / `ParamInt64` / `ParamFloat` | path | `c.Param("id", "0")` |
| `Int64Param` | path obrigatório | `c.Int64Param("id")` |
| `Query` / `Int` / `Float` / `Bool` / `IntE` / `QuerySlice` / `Sort` | query | `c.Int("page", 1)` |
| `Header` / `Cookie` | meta | `c.Header("X-Tenant", "1")` |
| `Parse` / `BodyParser` / `BodyJSON` / `Bind` / `BindForm` | body | `nitro.Parse[T](c)` |
| `FormValue` / `FormFile` | multipart | `c.FormFile("avatar")` |
| `Body()` | raw | `[]byte` |

**Atenção:** valores de Param/Query vivem no pool do request — não guarde após o handler.

## Ver também

- [Responses](responses.md) · [Context cheatsheet](context.md) · [Validation](validation.md) · [Routing](routing.md)
