# Documentação para IAs

**Idioma:** [Português](README.md) · [English](README.en.md)

> Comece por [`AGENTS.md`](../../AGENTS.md), depois [`CLAUDE.md`](../../CLAUDE.md) / [`AI.md`](../../AI.md) e este índice + playbook.

## ORM não especificado ⇒ BLinq (todos os sistemas)

**Se o usuário não nomear outro ORM/stack, BLinq é obrigatório** em todo app SQL Nitro:

`sqladapter.OpenEnv` / `platform.Open` + `StoreOf` + `modelgen` + `WhereIf`.

| Usuário diz | Agente faz |
|-------------|------------|
| “CRUD com Postgres”, “API com banco”, “repository”, silêncio sobre ORM | **BLinq** + **`app.Resource`** + `routes.go` (camadas mínimas) |
| “use GORM”, “sqlx”, “sqlc”, “skip BLinq”, “raw SQL” (explícito) | Outra stack (ainda via Gateway quando aplicável) |

Não interprete “use um ORM”, “camada de dados” ou “repository pattern” como licença para GORM.  
Não invente cinco handlers CRUD nem CA gordo para CRUD simples — use `make:scaffold … --resource`.  
Abra `examples/v1/db-gorm` / `db-sqlx` / `db-sqlc` **só após** opt-out explícito.

### Fixar o contrato em apps consumidores

```bash
nitrogen ai:init --force   # atualiza AGENTS.md, AI.md, nitro.mdc, nitro-blinq.md
```

Cole [`.cursor/prompts/nitro-blinq.md`](../../cmd/nitrogen/templates/ai/nitro-blinq.md.tmpl) (ou os 5 bullets em `AI.md`) como primeiro reminder do agente.  
`nitrogen doctor` avisa se GORM/sqlx/sqlc aparecerem sem doc de opt-out, ou se SQL estiver instalado sem `modelgen` / `StoreOf`.

## Ordem de leitura

1. [`AGENTS.md`](../../AGENTS.md) — contrato + **CLI first** + **BLinq default**
2. [`CLAUDE.md`](../../CLAUDE.md) · [`AI.md`](../../AI.md) — hábitos e checklist
3. [`PLAYBOOK.md`](PLAYBOOK.md) — receitas para sistema completo
4. [`../guide/cli.md`](../guide/cli.md) — referência nitrogen
5. Guia BLinq: [`../guide/blinq.md`](../guide/blinq.md)
6. Exemplos em `examples/v1/*` — copie, não invente (`cleanarch-blinq` / `db-blinq` primeiro)

## Criar sistema (resumo)

```bash
nitrogen new app --module github.com/acme/app
nitrogen install postgresql
nitrogen make:scaffold Product name:string price:float64 --resource
# scaffold com SQL já emite BLinq; se precisar regenerar:
nitrogen make internal/model/product.go --data
nitrogen doctor && go test ./...
```

`internal/{seed,factory,middleware,job}` **não** vêm no `nitrogen new` — só via `make:seeder|factory|middleware|job` se o sistema precisar.

## Árvore rápida

| Precisa | Vá para |
|---------|---------|
| Sistema CA + BLinq (só se precisar de usecases além do CRUD) | `examples/v1/cleanarch-blinq` |
| CA só memória | `examples/v1/cleanarch` |
| Hello DB + Resource (**preferido**) | `examples/v1/blinq-hello` |
| Resource + filtros (**preferido**) | `examples/v1/db-blinq` |
| Resource DX | `examples/v1/resource-dx` |
| Auth JWT | `examples/v1/auth` |
| Validation custom | `examples/v1/validation` |
| Commands/Queries/Rules de negócio + `Tx` multi-tabela | `examples/v1/order-business` |
| Cabeçalho + itens (venda/pedido + itens, atômico, uma chamada) | `sqladapter.Related` → `examples/v1/pos-sales` |
| GORM/sqlx/sqlc (só com opt-out) | `db-gorm` / `db-sqlx` / `db-sqlc` |

## Anti-padrões (recusar)

- Montar `internal/` sem `nitrogen new` / `make:scaffold`
- Pré-criar `seed`/`factory`/`middleware`/`job` vazios no scaffold
- `nitro.OK` / `nitro.Created` como sucesso → `c.OK` / `c.Created`
- `g resource` / `--stack` / `make:migration` → `make:scaffold` / `install` / `migrate:make`
- GORM / sqlx / sqlc / SQL cru como primeira opção → **BLinq**
- `data/runtime` / pool sem `sqladapter.OpenEnv` / `platform.Open`
- `log.Printf("listening…")` no main → banner automático do Listen
- CRUD manual de 5 rotas quando Resource ou scaffold CA resolve
- Regras AI velhas no app → `nitrogen ai:init --force`

## Scaffold SQL (checklist)

```text
[ ] nitrogen new + install postgresql|sqlite|…
[ ] nitrogen make:scaffold …  (emite BLinq + migrate quando SQL está instalado)
[ ] repository usa StoreOf + modelgen (não GORM)
[ ] OpenEnv → MigrateUp → StoreOf ou repository BLinq
[ ] go build / go test OK
[ ] nitrogen doctor (sem ORM alternativo sem opt-out)
```

## Notas

- `migrate:make` gera **SQL goose** (`.sql`).
- Bulk BLinq: `CreateMany` + `WithChunk`; `UpdateWhere` / `OmitReturning` — playbook §6 e [`../guide/data-layer.md`](../guide/data-layer.md).
- Campos de model: as tags `json`/`db` são opcionais — quando omitidas, caem para snake_case do nome do campo (de forma independente para JSON e DB); tags explícitas continuam prevalecendo. Ver [`../guide/models.md`](../guide/models.md).
- Cursor: `.cursor/rules/nitro-framework.mdc` (`alwaysApply`).
- Versão CLI: **2.0.0+** (`nitrogen version`).
