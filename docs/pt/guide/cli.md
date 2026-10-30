# CLI — nitrogen (v1.4)

**Idioma:** [Português](cli.md) · [English](../../guide/cli.md)

Ferramenta oficial para scaffold e DX do Nitro. Binário: `cmd/nitrogen`.

> **Padrão de dados:** apps SQL usam **BLinq** via `sqladapter.OpenEnv` (`NITRO_DB_DRIVER` + `DATABASE_URL` / `DB_DSN`). Postgres usa pgx por baixo; MySQL/MariaDB, SQL Server e SQLite usam `database/sql`. `nitrogen make … --data` gera `modelgen`. Não gere SQL cru nem escolha outra ORM a menos que o requisito peça.

```bash
go install ./cmd/nitrogen   # ou: go run ./cmd/nitrogen …
nitrogen version            # 2.0.0+
```

IAs: ao **criar um sistema**, use estes comandos antes de escrever arquivos à mão — ver [`AGENTS.md`](../../../AGENTS.md).

## Projeto

| Comando | Descrição |
|---------|-----------|
| `nitrogen new <name>` | Clean Arch: `cmd/server` + `internal/{config,model,usecase,handler,repository}` + migrations + envs + Dockerfile + Makefile. **Não** cria `seed`/`factory`/`middleware`/`job` |
| `nitrogen new <name> --basic` | Só `main.go` mínimo |
| `nitrogen new <name> --microservice` | CA + `docker-compose` + health/service name |
| `nitrogen new <name> --worker` | Worker AMQP: `cmd/worker` + `internal/consumer` + health `:8081` |
| `nitrogen new <name> --fleet --services a,b` | Monorepo: `go.work` + compose + Makefile; `gateway`→HTTP, demais→workers |
| `--module <path>` | Caminho do módulo Go |
| `--nitro-path <dir>` | `replace` local do monorepo Nitro (opcional: auto-detecta checkout pai ou `NITRO_PATH`) |

Arquivos de ambiente gerados: `.env` e `.env.example` apenas.  
Também: `.gitignore`, `.dockerignore`, e **regras para IAs** (`AGENTS.md`, `CLAUDE.md`, `AI.md`, `.cursor/rules/nitro.mdc`, `.cursor/prompts/nitro-blinq.md`) em todos os tiers.

## Agentes de IA

```bash
nitrogen ai:init              # diretório atual
nitrogen ai:init --dir ./app
nitrogen ai:init --force      # sobrescreve (atualiza política BLinq STOP)
```

Escreve contrato **consumidor** (inglês): **ORM não especificado ⇒ BLinq**, **`app.Resource` + `routes.go`**, camadas mínimas, `Parse`, `c.OK`, CLI-first. Inclui reminder em `.cursor/prompts/nitro-blinq.md`. `nitrogen new` roda isso automaticamente. Idempotente sem `--force`. `nitrogen doctor` avisa GORM/sqlx/sqlc sem opt-out.

## Install (drivers)

```bash
nitrogen install postgresql   # aliases: postgres, pg
nitrogen install mysql | mariadb | sqlserver | sqlite
nitrogen install valkey        # fork BSD-3 do Redis 7.2 (recomendado)
nitrogen install redis
nitrogen install rabbitmq
nitrogen install minio
nitrogen install auth-jwt       # aliases: jwt, auth
nitrogen install auth-keycloak  # alias: keycloak
```

Efeitos:

1. Structs em `internal/config` (`DBConfig`, `RedisConfig`, `JWTSecret` / `KeycloakConfig`, …)
2. `NITRO_DB_DRIVER` + `DB_DSN` / `DATABASE_URL` em `.env` / `.env.example` (SQL); `JWT_SECRET` ou `KEYCLOAK_*` para auth
3. SQL + Redis L2 opcional no `cmd/server` (`sqladapter.OpenEnv`, `VALKEY_ADDR` ou `REDIS_ADDR` → `nitro.ValkeyClientFromEnv` → `SetCache`); `internal/platform` só para helpers rabbitmq/minio
4. Serviço em `docker-compose.yml` (sqlite não usa compose)
5. `replace`s locais no `go.mod` quando há `--nitro-path` / replace de `pkg`
6. `cmd/server/main.go` com `MigrateUp` + health ping (SQL)
7. **Auth:** `internal/auth/{jwt,keycloak}.go` + rota protegida de exemplo (`/profile` ou `/api/me`) no `main`

### Troca de engine (SQL)

Rodar de novo `nitrogen install <outro-sql>` **atualiza** (upsert) driver/DSN, avisa se `migrations/` pode ser específica do dialeto e comenta serviços SQL rivais no `docker-compose.yml`.

```bash
nitrogen install postgresql
# … depois, testes locais sem Docker:
nitrogen install sqlite
nitrogen doctor --db
```

### OpenAPI

```bash
nitrogen openapi                     # grava openapi.json a partir do scan estático
nitrogen openapi --bearer -o docs/openapi.json
```

Detecta `Get`/`Post`/…, `Resource`, `AuthRoutes` / `KeycloakAuthRoutes` (mesmo scanner de `nitrogen routes`). Em runtime, prefira `app.OpenAPI(...)` / `app.Swagger(...)` depois de registrar as rotas.

## Geradores `make:*`

| Comando | Efeito |
|---------|--------|
| `make:scaffold Product name:string price:float64` | **CA (padrão):** model + usecases + handlers + repo. SQL → BLinq + migrate + `platform.Open` |
| `make:scaffold … --resource` | **Modo Resource:** model + `app.Resource` (sem 5 usecases). Também automático se o app **não** tem `internal/` (ex.: `nitrogen new --basic`) **e** SQL está instalado |
| `make:scaffold … --with-seed` / `--with-factory` | Também roda `make:seeder` / `make:factory` |
| `make internal/model/product.go --data` | **BLinq**: DSL em `internal/modelgen/` — **padrão com Postgres** |
| `make … --data --same-package` | Legado: DSL + `TableName` no mesmo pacote (`*_model_gen.go`) |
| `make:model Product` | Struct vazia |
| `make:model Product -r` / `--resource` | Model CoC (`nitro.Model`, tags, `//nitro:table=`) |
| `make:handler` / `make:usecase` / `make:repository` | Camada isolada (`make:usecase Product [OpName]` para CRUD ou usecase avulso) |
| `make:middleware Auth` | Cria `internal/middleware/` (on-demand) + stub; remove `.gitkeep` se existir |
| `make:job SyncOrders` | Cria `internal/job/` (on-demand) + stub schedule |
| `make:seeder` / `make:factory` / `make:test` | Cria `internal/seed` ou `factory` só quando chamado; testes |
| `make:relation A belongs_to\|has_many B` | FK no model filho + `modelgen/relations_gen.go` + stub de migration |
| `make:dto` / `make:controller` | DTO público / controller a partir de model |
| `model:show Product` | Lista campos e tags |

**CA vs Resource:** use `--resource` para CRUD de tabela; mantenha CA quando precisar de usecases/regras de domínio.

**Nomes:** `products`, `Product` ou `Products` → tipo `Product`, pacote `product`, tabela `products`.

Pastas `internal/{seed,factory,middleware,job}` **não** vêm no `nitrogen new`. Só são criadas pelos `make:*` correspondentes (ou se o requisito do sistema precisar). Ao gravar o primeiro arquivo real num diretório que tenha `.gitkeep`, a CLI remove o `.gitkeep`.

## Migrations

```bash
nitrogen migrate:make internal/model/product.go [--name create_products]
nitrogen migrate up          # via goose, se instalado
```

Não use `make:migration` (removido).

## Runtime / qualidade

| Comando | Uso |
|---------|-----|
| `up` | `docker compose up -d` (se existir) → espera DB → `MigrateUp` → `go run ./cmd/server` (`--no-dev`, `--seed`, `--migrate-only`) |
| `down` | `docker compose down` (`--volumes` opcional) |
| `dev` | Hot reload (`air` se disponível) |
| `routes` | Lista rotas |
| `openapi` | Grava `openapi.json` a partir do scan estático (`--bearer`, `-o`) |
| `test` / `cover` | Testes e coverage |
| `doctor` | Go/git + layout + regras IA; Go 1.25+ com SQL; avisa `*_tablename_gen.go` |
| `doctor --db` | Ping via `sqladapter.OpenEnv` |
| `seed` | Executa `cmd/seed` (gera a partir de `Seed*` se faltar) |
| `audit` / `security` / `stress` / `profile` | Ops |

Depois de install + scaffold:

```bash
nitrogen up
```

## Exemplo gerado

Ver [`examples/v1/cleanarch`](../../examples/v1/cleanarch/) (memória) e [`examples/v1/cleanarch-blinq`](../../examples/v1/cleanarch-blinq/) (Postgres + BLinq `products`) — `new` + `install postgresql` + `make:scaffold` + `make … --data`.

## Listen (runtime Nitro)

`app.Listen(":8080")` imprime o banner (com **PORT**) e `Listening on http://…`. No graceful shutdown: `Nitro: shutting down…` / `Nitro: server stopped`. Não duplique esses logs no `main`.
