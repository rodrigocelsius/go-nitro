# Documentação para IAs

**Idioma:** [Português](README.md) · [English](../../ai/README.en.md)

> Comece por [`AGENTS.md`](../../../AGENTS.md), depois [`CLAUDE.md`](../../../CLAUDE.md) / [`AI.md`](../../../AI.md) e este índice + playbook.

## ORM não especificado ⇒ BLinq (todos os sistemas)

**Se o usuário não nomear outro ORM/stack, BLinq é obrigatório** em todo app SQL Nitro.

| Usuário diz | Agente faz |
|-------------|------------|
| “CRUD com Postgres”, “repository”, silêncio sobre ORM | **BLinq** + **`app.Resource`** + `routes.go` |
| “use GORM”, “skip BLinq”, etc. (explícito) | Outra stack |

Fixar regras: `nitrogen ai:init --force`. Cole `.cursor/prompts/nitro-blinq.md`.  
Scaffold padrão: `make:scaffold … --resource`. CA só se houver regras além do CRUD.  
`nitrogen doctor` avisa ORM alternativo sem opt-out ou SQL sem `modelgen` / `StoreOf`.

Índice canônico (EN/PT): [`../../ai/README.md`](../../ai/README.md).

## Ordem de leitura

1. [`AGENTS.md`](../../../AGENTS.md) — contrato + **CLI first** + **BLinq default**
2. [`CLAUDE.md`](../../../CLAUDE.md) · [`AI.md`](../../../AI.md) — hábitos e checklist
3. [`PLAYBOOK.md`](PLAYBOOK.md) — receitas para sistema completo
4. [`../guide/cli.md`](../guide/cli.md) — referência nitrogen
5. Guia BLinq: [`../guide/blinq.md`](../guide/blinq.md)
6. Exemplos em `examples/v1/*` — copie `cleanarch-blinq` / `db-blinq` primeiro

## Criar sistema (resumo)

```bash
nitrogen new app --module github.com/acme/app
nitrogen install postgresql
nitrogen make:scaffold Product name:string price:float64
nitrogen make internal/model/product.go --data
nitrogen doctor && go test ./...
```

## Anti-padrões (recusar)

- GORM / sqlx / sqlc / SQL cru como primeira opção → **BLinq**
- Regras AI velhas → `nitrogen ai:init --force`
