# Config a partir do env (`ConfigOf`)

**Idioma:** [Português](config.md) · [English](../../guide/config.md)

Carregue as configurações da aplicação a partir do ambiente do processo e, opcionalmente, de `.env` / `.env.local`.

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

`ConfigOf` sempre chama `LoadDotEnv()` primeiro (por padrão **não** sobrescreve variáveis de ambiente já existentes).

## Tags

| Tag | Efeito |
|-----|--------|
| `env:"NAME"` | Chave de env (padrão: `FIELD_NAME` UPPER_SNAKE) |
| `env:"A,B"` | **Aliases** — o primeiro valor não vazio vence (`DB_DSN,DATABASE_URL`) |
| `env:"NAME,overwrite"` | Prefere o valor de `.env` / `.env.local` ao env do processo |
| `env:"DB_"` em um campo **struct** | Prefixo para os filhos (`HOST` → `DB_HOST`) |
| `default:"…"` | Usado quando o env está vazio / ausente |
| `validate:"required"` | Ausente → erro (também roda o validate completo da struct) |

## Campos tipados

| Tipo | Parse |
|------|--------|
| `string`, ints, floats, bool | Padrão |
| `[]string` | CSV (`a,b,c`) |
| `time.Duration` | `5s`, `1h`, … |
| `url.URL` / `*url.URL` | URL absoluta |

```go
Timeout time.Duration `env:"HTTP_TIMEOUT" default:"30s"`
Base    url.URL       `env:"API_URL,overwrite"`
```

Overwrite global de todas as chaves a partir do `.env`:

```go
nitro.LoadDotEnv(nitro.WithOverwrite())
```


## Structs aninhadas

Campos exportados de struct são preenchidos recursivamente. Os filhos normalmente usam nomes `env` absolutos:

```go
type Config struct {
    Redis RedisConfig
}
type RedisConfig struct {
    Addr string `env:"REDIS_ADDR" default:"localhost:6379"`
}
```

Aninhamento com prefixo:

```go
type Config struct {
    DB Nest `env:"DB_"`
}
type Nest struct {
    Host string `env:"HOST"` // → DB_HOST
}
```

Embeds anônimos são achatados no prefixo atual.

## `[]string` (CSV)

```go
Origins []string `env:"CORS_ALLOWED_ORIGINS" default:"http://localhost:3000"`
// CORS_ALLOWED_ORIGINS=a, b → []string{"a","b"}
```

## Não faça

- Reimplementar parsers de `.env` na app
- Achatar config aninhada numa segunda camada de mapeamento “flatEnv” — use tags aninhadas

Veja também: [`docs/api/config.md`](../api/config.md) (`nitro.Config` do servidor).
