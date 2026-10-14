# Validation

**Idioma:** [Português](validation.md) · [English](../../guide/validation.md)

Nitro valida structs com tags `validate` (bind HTTP via `BodyParser` / `BodyJSON` / `Parse` / Resource).
`Parse` e `BodyParser` aceitam JSON **ou** form conforme `Content-Type`.

O engine fica em `usecases/validator`. Regras de **documento BR** (CPF, CNPJ, phone, CEP, UF) são um **plugin** em `usecases/validator/br` — **opt-in** via `nitro.RegisterBRValidators`. Não entram em `NewValidator()`.

## Onde cada extra vive

| Artefato | Onde | Como entra |
|----------|------|------------|
| `email`, `uuid`, `min`, `e164` | `usecases/validator` builtins | `NewValidator()` |
| `cpf`, `cnpj`, `phone_br`, `cep`, `uf` | `usecases/validator/br` | `nitro.RegisterBRValidators(v)` |
| Regras da empresa (`sku`, …) | app (`internal/validation`) | `v.Register("sku", …)` |
| `digits`, `trim`, `titlecase=pt` | `usecases/transform` | tag `transform:` no Parse |
| `Digits` / `IsDigits` / `NormalizeEmail` | transform + `pkg` | chamada direta |
| `FormatMoney` / `FormatDate` / `SetLocale` | `domain/money`, `temporal`, `locale` | API de valor + locale |

Novas regras de **documento BR** (IE, PIS, CNH, PIX) entram no mesmo plugin `validator/br`. Formatação de dinheiro/data **não** mistura com `RegisterBRValidators`.

## Quick start

```go
type User struct {
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"gte=18"`
}

app.Post("/users", func(c *nitro.Ctx) error {
    user, err := nitro.Parse[User](c)
    if err != nil {
        return err // 400 JSON inválido, 422 validação
    }
    return c.Created(user)
})
```

Resposta 422:

```json
{"error":"validation failed","fields":{"email":"is required"}}
```

## Registry por App

```go
v := nitro.NewValidator()          // builtins isolados
nitro.RegisterBRValidators(v)      // cpf, cnpj, phone_br, cep, uf
v.Register("sku", SKUValidator)
app := nitro.New(nitro.Config{Validator: v})
// ou: app.SetValidator(v)
```

Cada `nitro.New()` já traz um validator próprio. Preferir `Register` na instância da app (não no global) para não vazar regras entre apps/testes.

```go
validation.Register(app.Validator()) // ver examples/v1/validation
```

`StrictUnknown` (default **true**): tag com regra não registrada → `Rule: "unknown"`.

Opt-out: `v.StrictUnknown = false` no `*StructValidator`.

## Mensagens customizadas

Prioridade:

1. `msg_<rule>:"…"` (ex.: `msg_cpf`, `msg_required`)
2. `message:"…"`
3. Default da regra em inglês

Placeholders em mensagens custom: `{field}`, `{param}`, `{value}`, `{rule}`.

```go
CPF string `validate:"required,cpf" msg_cpf:"Informe um CPF válido" message:"CPF obrigatório ou inválido"`
```

## Defaults (`default`)

Campos omitidos no JSON recebem valor padrão **antes** da validação (em `Parse` / `BodyJSON`):

```go
Active bool `json:"active" default:"true"`
// ou: validate:"default=true"
```

`"active": false` explícito **não** é sobrescrito. Funciona em slices (`[]Product` no bulk).

## Transforms (`transform`)

Pipeline: **decode → default → transform → validate**.

Tags `transform:"…"` (pipe por vírgula) mutam `string` / `*string` antes da validação:

| Regra | Exemplo | Efeito |
|-------|---------|--------|
| `trim` | `"  a "` → `"a"` | trim |
| `upper` / `lower` | | case |
| `digits` | `"(11) 9"` → `"119"` | só dígitos |
| `titlecase` | `"joão silva"` → `"João Silva"` | title |
| `titlecase=pt` | `"joão da silva"` → `"João da Silva"` | PT (de/da/do…) |
| `camelcase` / `pascalcase` | `foo_bar` | camel / Pascal |
| `snake` / `kebab` | `FooBar` | snake / kebab |
| `squeezespace` | espaços colapsados | |

```go
Name  string  `json:"name" validate:"required,min=3" transform:"trim,titlecase=pt"`
Phone string  `json:"phone" validate:"required" transform:"digits"`
State string  `json:"state" transform:"trim,upper"`
Nick  *string `json:"nick" transform:"titlecase=pt"`
```

Helpers: `nitro.ApplyTransforms(v)`, `nitro.TitleCase`, `nitro.Digits`, `nitro.CamelCase`, …

**Não** reinventar `toTitleCase` / `digitsOnly` / `isDigits` / `splitName` / `randomDigits` no app — use a tag ou os helpers da fachada.

### Helpers de texto / código (fachada `nitro`)

Uso em usecases (além das tags `transform` / `validate`):

| Função | Uso |
|--------|-----|
| `Digits` | só ASCII 0-9 (`"(11) 9"` → `"119"`) |
| `IsDigits` | código numérico (`"123456"`) |
| `SplitName` | `"Ana da Silva"` → `("Ana", "da Silva")` |
| `NormalizeEmail` | trim + lowercase |
| `IsEmail` | mesmas regras de `validate:"email"` |
| `RandomDigits(n)` | OTP / convite de n dígitos (`crypto/rand`) |
| `RandomAlphaNum(n)` / `RandomHex(nBytes)` | tokens / secrets |
| `IfBlank` | fallback se vazio |
| `Ptr` / `Deref` / `Contains` | ponteiro e slice |

```go
phone := nitro.Digits(in.Phone)
if !nitro.IsDigits(code) || len(code) != 6 {
    return nitro.BadRequest("código deve ter 6 dígitos")
}
email := nitro.NormalizeEmail(in.Email)
if !nitro.IsEmail(email) {
    return nitro.UnprocessableEntity("email inválido")
}
first, last := nitro.SplitName(name)
code, err := nitro.RandomDigits(6)
```

Exemplo: `examples/v1/transform`.

## Alias `same`

`same=Password` ≡ `eqfield=Password` (também `eq_field`).

```go
Password string `json:"password" validate:"required"`
Confirm  string `json:"confirm" validate:"same=Password"`
```

## Regras builtin (resumo)

| Grupo | Regras |
|-------|--------|
| Presença | `required`, `omitempty`†, `dive`†, `required_if`, `required_unless`, `required_with` |
| Comparação | `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `eqfield` / **`same`**, `nefield`, `gtfield`… |
| Tamanho | `min`, `max`, `len` |
| Conjunto | `in`, `oneof`, `not_in`, `unique` |
| Formato | `email`, `uuid`/`uuid4`/`uuid7`, `url`/`uri`, `alpha`/`alphanum`/`numeric`, `startswith`/`endswith`, `contains`/`excludes`, `datetime` |
| Rede | `ip`, `ipv4`, `ipv6`, `cidr`, `e164` |

† Meta: não são `ValidatorFunc`; o engine trata.

Nested: `address.street`. Slice: `validate:"dive,min=2"` (elementos). Cross-field: `required_with`, `required_if`, `eqfield` / alias `same`.

`Validate()` no model roda depois das tags (regras de negócio).

## Clean Architecture

```text
app
  nitro.NewValidator()            ← builtins (email, uuid, min, …)
  nitro.RegisterBRValidators(v)   ← plugin BR (cpf/cnpj/phone_br/cep/uf)
  v.Register("sku", …)            ← domínio da empresa (nunca no framework)
usecases/validator/br/            ← implementação do plugin
usecases/validator/               ← builtins + engine
usecases/transform/               ← tags transform: (titlecase=pt, digits)
domain/                           ← ValidationError; money/temporal/locale (format)
```

Exemplo completo: [`examples/v1/validation`](../../examples/v1/validation/).

## API pública (`nitro`)

| Função | Uso |
|--------|-----|
| `NewValidator()` | Instância com builtins |
| `RegisterBRValidators(v)` | CPF / CNPJ (numérico **e** alfanumérico RFB 2026) / phone_br / cep / uf |

| `Register` / `RegisterAlias` no validator | Regras / aliases da app |
| `SetOptions(Options{StopOnFirstError, StrictUnknown})` | Comportamento |
| `ParseRules(tag)` | Debug / tooling |
| `Validate(v)` | Sem HTTP (default process-wide) |
| `app.SetValidator` / `Validator()` | Wiring |

Metadados de struct (`reflect.Type` → campos + tags) são cacheados em `sync.Map` (ver `BenchmarkValidateStruct`).

## Pacote interno

`github.com/rodrigocelsius/go-nitro/usecases/v2/validator` — engine. A fachada `usecases.ValidateStruct` / `nitro.Validate` permanece estável.
