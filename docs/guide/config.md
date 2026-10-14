# Config from env (`ConfigOf`)

**Language:** [English](config.md) · [Português](../pt/guide/config.md)

Load application settings from process environment and optional `.env` / `.env.local`.

```go
type Config struct {
    App AppConfig
    DB  DBConfig
}

type AppConfig struct {
    Name string `env:"APP_NAME" default:"my-api"`
    Port int    `env:"APP_PORT" default:"8080"`
}

type DBConfig struct {
    DSN string `env:"DB_DSN,DATABASE_URL" validate:"required"`
}

cfg, err := nitro.ConfigOf[Config]()
// or: cfg := nitro.MustConfig[Config]()
```

`ConfigOf` always calls `LoadDotEnv()` first (does **not** overwrite existing env vars by default).

## Tags

| Tag | Effect |
|-----|--------|
| `env:"NAME"` | Env key (default: `FIELD_NAME` UPPER_SNAKE) |
| `env:"A,B"` | **Aliases** — first non-empty wins (`DB_DSN,DATABASE_URL`) |
| `env:"NAME,overwrite"` | Prefer value from `.env` / `.env.local` over process env |
| `env:"DB_"` on a **struct** field | Prefix for children (`HOST` → `DB_HOST`) |
| `default:"…"` | Used when env empty / unset |
| `validate:"required"` | Missing → error (also runs full struct validate) |

## Typed fields

| Type | Parse |
|------|--------|
| `string`, ints, floats, bool | Standard |
| `[]string` | CSV (`a,b,c`) |
| `time.Duration` | `5s`, `1h`, … |
| `url.URL` / `*url.URL` | Absolute URL |

```go
Timeout time.Duration `env:"HTTP_TIMEOUT" default:"30s"`
Base    url.URL       `env:"API_URL,overwrite"`
```

Global overwrite of all keys from `.env`:

```go
nitro.LoadDotEnv(nitro.WithOverwrite())
```


## Nested structs

Exported struct fields are filled recursively. Children normally use absolute `env` names:

```go
type Config struct {
    Redis RedisConfig
}
type RedisConfig struct {
    Addr string `env:"REDIS_ADDR" default:"localhost:6379"`
}
```

Prefix nesting:

```go
type Config struct {
    DB Nest `env:"DB_"`
}
type Nest struct {
    Host string `env:"HOST"` // → DB_HOST
}
```

Anonymous embeds are flattened into the current prefix.

## `[]string` (CSV)

```go
Origins []string `env:"CORS_ALLOWED_ORIGINS" default:"http://localhost:3000"`
// CORS_ALLOWED_ORIGINS=a, b → []string{"a","b"}
```

## Do not

- Reimplement `.env` parsers in the app
- Flatten nested config into a second “flatEnv” mapping layer — use nested tags

See also: [`docs/api/config.md`](../api/config.md) (server `nitro.Config`).
