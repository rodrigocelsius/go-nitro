# Guides

**Language:** [English](README.md) · [Português](../pt/guide/README.md)

Human docs for Nitro v1. For agents: [PLAYBOOK (EN)](../ai/PLAYBOOK.en.md) · [Playbook (PT)](../ai/PLAYBOOK.md) · [`../../AGENTS.md`](../../AGENTS.md).

## Getting started

| Guide | Contents |
|------|----------|
| [Welcome](welcome.md) | Install and hello world |
| [philosophy](philosophy.md) | Nitro v1 philosophy and legacy migration |
| [Cache](cache.md) | L1/L2 resource cache, replicas, tenants |
| [Production](production.md) | Checklist: what to set, what is verified |
| [Conventions](conventions.md) | CoC contract: defaults, tenant, IP, migration table |
| [api changelog](api-changelog.md) | Legacy → v1 table |

## HTTP

| Guide | Contents |
|------|----------|
| [Routing](routing.md) | Routes, groups, Mount, Static/SPA, Hooks, reverse URL |
| [Request](request.md) | Path, query, CT-aware body, form |
| [Responses](responses.md) | OK, Created, File, Stream… |
| [Errors](errors.md) | Errors and status codes |
| [Context](context.md) | Ctx cheatsheet |
| [Middleware](middleware.md) | Defaults and opt-in |
| [Recipes](recipes.md) | Pagination, upload, SSE, WS, Resource |
| [Business Resource](business-resource.md) | Commands, Queries, Rules on `ResourceConfig` (v2) |
| [Resource hooks](resource-hooks.md) | `AfterCreate`, `BeforeUpdate`, … |

## Domain

| Guide | Contents |
|------|----------|
| [Model field tags](models.md) | Optional `json`/`db` tags, default snake_case naming |
| [Validation](validation.md) | Tags, Parse, registry, BR plugin |
| [Config (env)](config.md) | Nested `ConfigOf`, CSV, aliases |
| [Authentication](authentication.md) | JWT, sessions, OAuth2 |
| [BFF](bff.md) | Backend-for-frontend |
| [Data layer](data-layer.md) | **BLinq default**: Store, Gateway, Resource |
| [BLinq](blinq.md) | Typed DSL (`modelgen`), migrate, codegen — **default with Postgres** |

## Ops

| Guide | Contents |
|------|----------|
| [CLI (nitrogen)](cli.md) | `new`, `install`, `make:*`, `migrate`, doctor |
| [Messaging](messaging.md) | In-memory / RabbitMQ queue, workers, fleet, resilience |
| [Testing](testing.md) | TestClient |
| [Deployment](deployment.md) | Deploy |
| [Benchmarks](../extra/benchmarks.md) | Current numbers |

> HTTP/DB/auth guides live in `docs/guide/` (this index).
