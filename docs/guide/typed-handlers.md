# Typed handlers, deprecation and Problem Details

## `nitro.Handle` — typed in/out

```go
type GetUser struct {
    ID int64 `validate:"required"` // bound from ":id" by convention
}

app.Get("/users/:id", nitro.Handle(func(ctx context.Context, in *GetUser) (*User, error) {
    return users.Find(ctx, in.ID) // returned value → JSON 200
}))

app.Post("/users", nitro.Handle(createUser, 201)) // optional success status
```

- Query parameters bind by convention too: a field without a tag reads `?<json-or-snake_case-name>` (`UnitPrice` ← `?unit_price=`); `query:"name"` overrides and `query:"-"` opts out.
- Input is bound from the JSON/form body or query (same rules as `nitro.Parse`), path params go into the field with the same JSON/snake_case name (`ID` ← `:id`; `param:"x"` overrides, `param:"-"` opts out), then `validate:` tags run.
- Return `nitro.Empty{}` (or a nil pointer) for `204 No Content`; use `nitro.Empty` as the input type when there is nothing to bind.
- `ctx` is the request context. Do not keep the `*nitro.Ctx` after returning; use `c.Detach()` for background work.

## Deprecating a version

```go
v1 := app.Group("/v1", nitro.Deprecated(time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC), "/v2"))
```

Adds `Deprecation: true`, `Sunset: <HTTP-date>` and `Link: </v2>; rel="successor-version"`.

## RFC 9457 / 7807 errors

```go
app := nitro.New(nitro.Config{ErrorHandler: nitro.ProblemDetails()})
```

Responses use `application/problem+json` (`type`, `title`, `status`, `detail`, `instance`, `request_id`); internal errors are never leaked.
