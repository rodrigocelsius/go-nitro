# Validation

**Language:** [English](validation.md) · [Português](../pt/guide/validation.md)

Nitro validates structs with `validate` tags (HTTP bind via `BodyParser` / `BodyJSON` / `Parse` / Resource).
`Parse` and `BodyParser` accept JSON **or** form according to `Content-Type`.

The engine lives in `usecases/validator`. **BR document** rules (CPF, CNPJ, phone, CEP, UF) are a **plugin** in `usecases/validator/br` — **opt-in** via `nitro.RegisterBRValidators`. They are not part of `NewValidator()`.

## Where each extra lives

| Artifact | Where | How it enters |
|----------|------|------------|
| `email`, `uuid`, `min`, `e164` | `usecases/validator` builtins | `NewValidator()` |
| `cpf`, `cnpj`, `phone_br`, `cep`, `uf` | `usecases/validator/br` | `nitro.RegisterBRValidators(v)` |
| Company rules (`sku`, …) | app (`internal/validation`) | `v.Register("sku", …)` |
| `digits`, `trim`, `titlecase=pt` | `usecases/transform` | `transform:` tag on Parse |
| `Digits` / `IsDigits` / `NormalizeEmail` | transform + `pkg` | direct call |
| `FormatMoney` / `FormatDate` / `SetLocale` | `domain/money`, `temporal`, `locale` | value API + locale |

New **BR document** rules (IE, PIS, CNH, PIX) go in the same `validator/br` plugin. Money/date formatting does **not** mix with `RegisterBRValidators`.

## Quick start

```go
type User struct {
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"gte=18"`
}

app.Post("/users", func(c *nitro.Ctx) error {
    user, err := nitro.Parse[User](c)
    if err != nil {
        return err // 400 invalid JSON, 422 validation
    }
    return c.Created(user)
})
```

422 response:

```json
{"error":"validation failed","fields":{"email":"is required"}}
```

## Per-app registry

```go
v := nitro.NewValidator()          // isolated builtins
nitro.RegisterBRValidators(v)      // cpf, cnpj, phone_br, cep, uf
v.Register("sku", SKUValidator)
app := nitro.New(nitro.Config{Validator: v})
// or: app.SetValidator(v)
```

Each `nitro.New()` already has its own validator. Prefer `Register` on the app instance (not the global) so rules do not leak across apps/tests.

```go
validation.Register(app.Validator()) // see examples/v1/validation
```

`StrictUnknown` (default **true**): tag with an unregistered rule → `Rule: "unknown"`.

Opt-out: `v.StrictUnknown = false` on `*StructValidator`.

## Custom messages

Priority:

1. `msg_<rule>:"…"` (e.g. `msg_cpf`, `msg_required`)
2. `message:"…"`
3. Default English rule message

Placeholders in custom messages: `{field}`, `{param}`, `{value}`, `{rule}`.

```go
CPF string `validate:"required,cpf" msg_cpf:"Enter a valid CPF" message:"CPF required or invalid"`
```

## Defaults (`default`)

Fields omitted from JSON receive a default **before** validation (in `Parse` / `BodyJSON`):

```go
Active bool `json:"active" default:"true"`
// or: validate:"default=true"
```

Explicit `"active": false` is **not** overwritten. Works on slices (`[]Product` in bulk).

## Transforms (`transform`)

Pipeline: **decode → default → transform → validate**.

`transform:"…"` tags (comma-separated) mutate `string` / `*string` before validation:

| Rule | Example | Effect |
|-------|---------|--------|
| `trim` | `"  a "` → `"a"` | trim |
| `upper` / `lower` | | case |
| `digits` | `"(11) 9"` → `"119"` | digits only |
| `titlecase` | `"joão silva"` → `"João Silva"` | title |
| `titlecase=pt` | `"joão da silva"` → `"João da Silva"` | PT (de/da/do…) |
| `camelcase` / `pascalcase` | `foo_bar` | camel / Pascal |
| `snake` / `kebab` | `FooBar` | snake / kebab |
| `squeezespace` | collapsed spaces | |

```go
Name  string  `json:"name" validate:"required,min=3" transform:"trim,titlecase=pt"`
Phone string  `json:"phone" validate:"required" transform:"digits"`
State string  `json:"state" transform:"trim,upper"`
Nick  *string `json:"nick" transform:"titlecase=pt"`
```

Helpers: `nitro.ApplyTransforms(v)`, `nitro.TitleCase`, `nitro.Digits`, `nitro.CamelCase`, …

**Do not** reinvent `toTitleCase` / `digitsOnly` / `isDigits` / `splitName` / `randomDigits` in the app — use the tag or the facade helpers.

### Text / code helpers (`nitro` facade)

Use in usecases (in addition to `transform` / `validate` tags):

| Function | Use |
|--------|-----|
| `Digits` | ASCII 0-9 only (`"(11) 9"` → `"119"`) |
| `IsDigits` | numeric code (`"123456"`) |
| `SplitName` | `"Ana da Silva"` → `("Ana", "da Silva")` |
| `NormalizeEmail` | trim + lowercase |
| `IsEmail` | same rules as `validate:"email"` |
| `RandomDigits(n)` | n-digit OTP / invite (`crypto/rand`) |
| `RandomAlphaNum(n)` / `RandomHex(nBytes)` | tokens / secrets |
| `IfBlank` | fallback if blank |
| `Ptr` / `Deref` / `Contains` | pointer and slice |

```go
phone := nitro.Digits(in.Phone)
if !nitro.IsDigits(code) || len(code) != 6 {
    return nitro.BadRequest("code must have 6 digits")
}
email := nitro.NormalizeEmail(in.Email)
if !nitro.IsEmail(email) {
    return nitro.UnprocessableEntity("invalid email")
}
first, last := nitro.SplitName(name)
code, err := nitro.RandomDigits(6)
```

Example: `examples/v1/transform`.

## `same` alias

`same=Password` ≡ `eqfield=Password` (also `eq_field`).

```go
Password string `json:"password" validate:"required"`
Confirm  string `json:"confirm" validate:"same=Password"`
```

## Builtin rules (summary)

| Group | Rules |
|-------|--------|
| Presence | `required`, `omitempty`†, `dive`†, `required_if`, `required_unless`, `required_with` |
| Comparison | `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `eqfield` / **`same`**, `nefield`, `gtfield`… |
| Size | `min`, `max`, `len` |
| Set | `in`, `oneof`, `not_in`, `unique` |
| Format | `email`, `uuid`/`uuid4`/`uuid7`, `url`/`uri`, `alpha`/`alphanum`/`numeric`, `startswith`/`endswith`, `contains`/`excludes`, `datetime` |
| Network | `ip`, `ipv4`, `ipv6`, `cidr`, `e164` |

† Meta: these are not `ValidatorFunc`; the engine handles them.

Nested: `address.street`. Slice: `validate:"dive,min=2"` (elements). Cross-field: `required_with`, `required_if`, `eqfield` / alias `same`.

`Validate()` on the model runs after tags (business rules).

## Clean Architecture

```text
app
  nitro.NewValidator()            ← builtins (email, uuid, min, …)
  nitro.RegisterBRValidators(v)   ← BR plugin (cpf/cnpj/phone_br/cep/uf)
  v.Register("sku", …)            ← company domain (never in the framework)
usecases/validator/br/            ← plugin implementation
usecases/validator/               ← builtins + engine
usecases/transform/               ← transform: tags (titlecase=pt, digits)
domain/                           ← ValidationError; money/temporal/locale (format)
```

Full example: [`examples/v1/validation`](../../examples/v1/validation/).

## Public API (`nitro`)

| Function | Use |
|--------|-----|
| `NewValidator()` | Instance with builtins |
| `RegisterBRValidators(v)` | CPF / CNPJ (numeric **and** alphanumeric RFB 2026) / phone_br / cep / uf |
| `Register` / `RegisterAlias` on the validator | App rules / aliases |
| `SetOptions(Options{StopOnFirstError, StrictUnknown})` | Behavior |
| `ParseRules(tag)` | Debug / tooling |
| `Validate(v)` | Without HTTP (process-wide default) |
| `app.SetValidator` / `Validator()` | Wiring |

Struct metadata (`reflect.Type` → fields + tags) is cached in `sync.Map` (see `BenchmarkValidateStruct`).

## Internal package

`github.com/rodrigocelsius/go-nitro/usecases/v2/validator` — engine. The `usecases.ValidateStruct` / `nitro.Validate` facade stays stable.
