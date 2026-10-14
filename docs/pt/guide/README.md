# Guides

**Idioma:** [Português](README.md) · [English](../../guide/README.md)

Documentação humana do Nitro v1. Para agentes: [`../ai/PLAYBOOK.md`](../ai/PLAYBOOK.md) · [`../../../AGENTS.md`](../../../AGENTS.md) · [`../../../CLAUDE.md`](../../../CLAUDE.md) · [`../../../AI.md`](../../../AI.md).

Validação (EN + PT): [English](../../guide/validation.md) · [Português](validation.md).

## Começar

| Guia | Conteúdo |
|------|----------|
| [Welcome](welcome.md) | Instalação e hello world |
| [philosophy](philosophy.md) | Filosofia Nitro v1 e migração do legado |
| [api changelog](api-changelog.md) | Tabela legado → v1 |

## HTTP

| Guia | Conteúdo |
|------|----------|
| [Routing](routing.md) | Rotas, grupos, Mount, Static/SPA, Hooks, reverse URL |
| [Request](request.md) | Path, query, body CT-aware, form |
| [Responses](responses.md) | OK, Created, File, Stream… |
| [Errors](errors.md) | Erros e status |
| [Context](context.md) | Cheatsheet do Ctx |
| [Middleware](middleware.md) | Defaults e opt-in |
| [Recipes](recipes.md) | Paginação, upload, SSE, WS, Resource |

## Domínio

| Guia | Conteúdo |
|------|----------|
| [Validation](validation.md) | Tags, Parse, registry ([EN](../../guide/validation.md) · [PT](validation.md)) |
| [Config (env)](config.md) | `ConfigOf` nested, CSV, aliases |
| [Authentication](authentication.md) | JWT, sessions, OAuth2 |
| [BFF](bff.md) | Backend-for-frontend |
| [Data layer](data-layer.md) | **Padrão BLinq**: Store, Gateway, Resource |
| [BLinq](blinq.md) | DSL tipada (`modelgen`), migrate, codegen — **padrão com Postgres** |

## Ops

| Guia | Conteúdo |
|------|----------|
| [CLI (nitrogen)](cli.md) | `new`, `install`, `make:*`, `migrate`, doctor |
| [Messaging](messaging.md) | Queue in-memory / RabbitMQ, workers, fleet, resilience |
| [Testing](testing.md) | TestClient |
| [Deployment](deployment.md) | Deploy |
| [Benchmarks](../extra/benchmarks.md) | Números atuais |

> A cópia canônica em inglês fica em `docs/guide/`. Este índice é a versão em português.
