# Request — parameters and body

**Language:** [English](request.md) · [Português](../pt/guide/request.md)

How to read path, query, headers, cookies, form, and JSON on `*nitro.Ctx`.

## Path

```go
app.Get("/users/:id", func(c *nitro.Ctx) error {
    id := c.Param("id")           // "" if missing
    id = c.Param("id", "0")       // default (Fiber-like)
    return c.OK(nitro.Map{"id": id})
})

app.Get("/items/:id", func(c *nitro.Ctx) error {
    id := c.ParamInt("id", 0)
    id64 := c.ParamInt64("id", 0) // default; never returns error
    price := c.ParamFloat("price", 0)
    return c.OK(nitro.Map{"id": id, "id64": id64, "price": price})
})

// Required (BadRequest if invalid) — preferred in DB handlers:
id, err := c.Int64Param("id")
if err != nil {
    return err
}
```

### `ParamInt64` vs `Int64Param`

| Method | Signature | When to use |
|--------|------------|-------------|
| `c.ParamInt64("id", 0)` | `int64` | safe default; no error |
| `c.Int64Param("id")` | `(int64, error)` | required ID in CRUD/Postgres |

Do not confuse with `c.Int` / `c.IntE` (query string).

## Query

```go
q := c.Query("q")                 // "" if missing
lang := c.Query("lang", "en")     // default
page := c.Int("page", 1)
perPage := c.Int("per_page", 20)
min := c.Float("min_price", 0)
active := c.Bool("active")        // true if "true"|"1"|"yes"
tags := c.QuerySlice("tag")       // ?tag=a&tag=b
field, desc := c.Sort("created_at") // ?sort=-created_at

page, err := c.IntE("page")       // error if missing/invalid
```

## Headers and cookies

```go
token := c.Header("Authorization")
ver := c.Header("X-API-Version", "1")

sid := c.Cookie("session_id")
c.SetCookie("session_id", token, 86400, "/", "", true, true)
c.DeleteCookie("session_id")
```

## Body (unified Content-Type)

**Hierarchy:**

1. `nitro.Parse[T](c)` — preferred (typed + validate → 400/422; JSON **or** form)
2. `c.BodyParser(&v)` — bind by Content-Type + validate
3. `c.BodyJSON(&v)` — JSON only + validate
4. `c.BindJSON(&v)` — **deprecated** (alias of BodyJSON)
5. `c.Bind(&v)` — decode without validate (JSON / form / query per CT)
6. `c.BindForm(&v)` — form only (`form:` / `json:` / field name)
7. `c.BindMap()` — decode into `map[string]any` (no validate)

```go
user, err := nitro.Parse[User](c)
if err != nil {
    return err
}
return c.Created(user)
```

`Parse` / `BodyParser` / `Bind` detect `Content-Type`:

| Content-Type | Behavior |
|--------------|---------------|
| `application/json` | JSON |
| `application/x-www-form-urlencoded` / `multipart/form-data` | form (`form:"…"`) |
| empty + JSON body | JSON |
| empty / no body | query (`query:"…"`) |

JSON defaults: tag `default:"true"` or `validate:"default=true"` — only if the key **did not** appear in the body.  
Details: [Validation](validation.md).

## Form / upload

```go
name := c.FormValue("name")
file, err := c.FormFile("avatar")

// Typed struct (also via Parse/BodyParser if CT is form):
var in struct {
    Name string `form:"name"`
    Age  int    `form:"age"`
}
_ = c.BindForm(&in)
```

## Summary

| Method | Kind | Example |
|--------|------|---------|
| `Param` / `ParamInt` / `ParamInt64` / `ParamFloat` | path | `c.Param("id", "0")` |
| `Int64Param` | required path | `c.Int64Param("id")` |
| `Query` / `Int` / `Float` / `Bool` / `IntE` / `QuerySlice` / `Sort` | query | `c.Int("page", 1)` |
| `Header` / `Cookie` | meta | `c.Header("X-Tenant", "1")` |
| `Parse` / `BodyParser` / `BodyJSON` / `Bind` / `BindForm` | body | `nitro.Parse[T](c)` |
| `FormValue` / `FormFile` | multipart | `c.FormFile("avatar")` |
| `Body()` | raw | `[]byte` |

**Note:** Param/Query values live in the request pool — do not keep them after the handler returns.

## See also

- [Responses](responses.md) · [Context cheatsheet](context.md) · [Validation](validation.md) · [Routing](routing.md)
