# Benchmarks

**Idioma:** [Português](benchmarks.md) · [English](../../extra/benchmarks.md)

Comparação Nitro × Fiber × `net/http` (stdlib).

## Ambiente (última execução)

- **CPU**: AMD Ryzen 7 8700G
- **OS**: Linux amd64
- **Go**: 1.24.0 (toolchain)
- **Nitro**: defaults desligados no bench (`DisableDefaults`, sem RequestID/headers) para comparação justa
- **Comando**:

```bash
cd bench
go test -run='^$' -bench='Benchmark(Nitro|Fiber|Stdlib)InProcess|BenchmarkFrameworks|BenchmarkConcurrent' -benchmem -benchtime=300ms -count=1 .

# RPS leve (GOMAXPROCS=2, 1.5s, conc 1/2/4) — ~15–20s no total:
go test -run='^TestRPSLight$' -v -count=1 .
```

## RPS Light (2026-07-22) — `TestRPSLight`

Hardware limitado: `GOMAXPROCS=2`, janela **1.5s**, concorrência **1 / 2 / 4**.

### HTTP real

| Conc | stdlib RPS | Nitro RPS | Fiber RPS | Nitro p99 |
|------|------------|-----------|-----------|-----------|
| 1 | ~34k | **~36k** | ~36k | ~69 µs |
| 2 | ~84k | **~97k** | ~95k | ~76 µs |
| 4 | ~83k | **~97k** | ~95k | ~174 µs |

### In-process (Handler fasthttp, sem TCP)

| Framework | RPS |
|-----------|-----|
| Nitro | ~11.3M |
| Fiber | ~13.8M |

**Leitura:** no TCP com 2 cores, Nitro e Fiber empatam (~36k→97k RPS). In-process Fiber costuma vencer (menos camadas no handler cru); use HTTP real para decisão de produto.

## Resultados (2026-07-22) — benches ns/op

### HTTP real (client → server TCP)

| Rota | stdlib | Nitro | Fiber |
|------|--------|-------|-------|
| `GET /` | 32.0 µs · 78 alloc | **29.1 µs** · 56 alloc | 29.8 µs · 57 alloc |
| `GET /hello/:name` | 35.9 µs · 81 | **30.1 µs** · 56 | 30.1 µs · 58 |
| `GET /health` | 32.0 µs · 78 | 30.0 µs · 56 | **29.6 µs** · 57 |
| `GET /ok` (JSON) | 32.4 µs · 79 | **30.1 µs** · 58 | 30.1 µs · 59 |

**Leitura:** Nitro ≈ Fiber (±3%), ambos ~8–15% mais rápidos que stdlib.

### Paralelo (`b.RunParallel`)

| Framework | ns/op | B/op | allocs |
|-----------|-------|------|--------|
| stdlib | ~3.7k | ~6.5k | 72 |
| Nitro | ~2.7k | ~4.0k | 51 |
| **Fiber** | **~2.6k** | ~4.1k | 52 |

**Leitura:** empate técnico (Fiber ~5% à frente nesta corrida).

### In-process (sem TCP)

| Cenário | Nitro | stdlib | Fiber (`app.Test`)* |
|---------|-------|--------|---------------------|
| `GET /` | **491 ns** · 9 alloc | 1740 ns · 18 | ~5.1 µs · 30 |
| `GET /hello` | 633 ns · 9 | 2022 ns · 21 | ~5.2 µs · 31 |
| `GET /ok` | 672 ns · 11 | 1924 ns · 19 | ~5.2 µs · 32 |

\*Fiber `app.Test` não é hot path fasthttp puro — preferir HTTP real / paralelo.

## Resultados (2026-07-17)

### HTTP real (client → server TCP)

| Rota | stdlib | Nitro | Fiber |
|------|--------|-------|-------|
| `GET /` | 32.2 µs · 69 alloc | **28.2 µs** · 47 alloc | **27.5 µs** · 48 alloc |
| `GET /hello/:name` | 31.9 µs · 72 | 28.1 µs · 47 | 29.4 µs · 49 |
| `GET /health` | 31.2 µs · 69 | 28.0 µs · 47 | 28.4 µs · 48 |
| `GET /ok` (JSON) | 32.2 µs · 70 | 28.6 µs · 49 | 28.3 µs · 50 |

**Leitura:** Nitro ≈ Fiber (±3%), ambos ~10–15% mais rápidos que stdlib e com ~30% menos allocs no caminho HTTP.

### Paralelo (`b.RunParallel`)

| Framework | ns/op | B/op | allocs |
|-----------|-------|------|--------|
| stdlib | ~3.3k | ~5.6k | 63 |
| **Nitro** | **~2.4k** | **~3.2k** | **42** |
| Fiber | ~2.4k | ~3.2k | 43 |

**Leitura:** Nitro ≈ Fiber no paralelo (empate técnico). O antigo “Fiber ~2×” era artefato de medição (um `http.Client`/`Transport` compartilhado entre alvos + sem warmup). Agora: client dedicado por framework, pool aquecido, `MaxIdleConnsPerHost` alto.

Também no servidor: `Serve()` usa `serverConfig()` completo (`Concurrency: 256*1024` como Fiber/fasthttp, `NoDefaultDate`/`NoDefaultServerHeader`, um defer recover+pool). Benches anteriores montavam `fasthttp.Server{Handler: app.Handler()}` cru.

### In-process (sem TCP)

| Cenário | Nitro | stdlib (`httptest`) | Fiber (`app.Test`) |
|---------|-------|-------------------|--------------------|
| `GET /` | **492 ns** · 9 alloc | 1757 ns · 17 | ~6.0 µs · 30* |
| `GET /hello` | 627 ns · 9 | 2045 ns · 20 | ~6.0 µs · 31* |
| `GET /ok` | 704 ns · 11 | 2001 ns · 18 | ~6.2 µs · 32* |

\*Fiber `app.Test` passa por adapter `net/http` — não é o hot path fasthttp puro; use HTTP real / paralelo para comparar Fiber de forma justa.

### Throughput histórico (RPS, 5s, `TestThroughput`)

| Concurrency | stdlib | Nitro | Fiber |
|-------------|--------|-------|-------|
| 1 | ~33k | ~35k | ~36k |
| 10 | ~120k | **~196k** | ~179k |
| 50 | ~256k | ~385k | **~397k** |
| 100 | ~325k | ~414k | **~421k** |

## O que já foi otimizado (histórico)

- Server timeouts + `DisableHeaderNamesNormalizing` + concurrency
- `b2s` no hot path; pool de `FastCtx`
- Recover + `pool.Put` num único defer
- **(2026-07-17)** `SetContentTypeBytes` em `Write` / `JSON` / `Text`
- **(2026-07-17)** `Concurrency: 256*1024` (antes `GOMAXPROCS*256`), `Serve()` / `FastHTTPServer()`, `NoDefault*`
- **(2026-07-17)** Bench paralelo justo (client por alvo + warmup)
- **(2026-07-22)** `Write`/`Bytes` via `SetBodyRaw` (como Fiber `Send`); `ResetHot`; `handlerHot` + `DisableInlineRecover`; benches com `GetDomain`

## O que ainda pode melhorar (priorizado)

1. **Paralelismo vs Fiber** — resolvido na medição; residual de engine é ruído (±5%).
2. **Query** — `Int`/`Peek` já low-alloc; Float/Bool/IntE/QuerySlice otimizados. Allocs restantes vêm do parse de URI no fasthttp no bench frio.
3. **JSON** — `AppendJSON` + `SetBodyRaw`; path stdlib usa buffer pool + `Encoder`. Preferir codegen/`Write` no hot path.
4. **Logger Defaults** — ~~4561 ns~~ → **~2800 ns** (append + stderr direto, skip health/metrics, sem wrap duplo). Opt-in: `Async: true`.

## Metodologia justa

```go
nitro.New(nitro.Config{
    DisableDefaults:      true,
    DisableRequestID:     true,
    DisableTelemetry:     true,
    DisableInlineRecover: true,
    Security:             nitro.SecurityConfig{DisableHeaders: true},
})
// Preferir app.Serve(ln) — não fasthttp.Server{Handler: app.Handler()} cru.
// Hot path estático: GetDomain + c.Write (SetBodyRaw).
```

Servers HTTP compartilhados via `sync.Once`. Paralelo: um `http.Client` por framework + warmup do pool.

## Validation (`BenchmarkValidateStruct` + HTTP)

### Rodar

```bash
# Engine isolado
cd usecases/validator && go test -bench=BenchmarkValidateStruct -benchmem -count=3

# HTTP in-process + MemStats
cd bench
go test -run=TestValidationMemorySample -v -count=1
go test -run='^$' -bench='BenchmarkValidation' -benchmem -benchtime=500ms -count=3
```

Arquivo: `bench/validation_bench_test.go`.

### Engine isolado (sem HTTP/JSON) — AMD Ryzen 7 8700G, 2026-07-18

| Caso | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| Pré type-meta cache (baseline Fase 8) | ~1470 | 696 | 29 |
| **Valid (com cache + RAM wins)** | **~470** | **48** | **1** |
| Invalid (coleta todos os erros) | ~960 | 328 | 10 |
| Invalid + `StopOnFirstError` | ~360 | 142 | 6 |

### HTTP in-process: Bind vs BodyJSON (custo da validação no request)

Mesmo JSON válido `{"name":"John","email":"john@example.com","status":"active"}`.

| Path | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| `POST /bind` (só `c.Bind`) | **~1700** | 2184 | 22 |
| `POST /validate` (`c.BodyJSON`) | **~2950** | 2595 | 44 |
| **Delta = validação** | **~+1250 (~+73%)** | **~+410** | **~+22** |
| `BodyJSON` inválido → 422 | ~4570 | 4095 | 69 |
| `BodyJSON` nested | ~3670 | 2732 | 50 |
| `GET /ping` (controle, sem body) | ~510 | 1634 | 9 |

**Leitura:** no hot path HTTP, a validação adiciona ~1.2 µs e ~22 allocs (~410 B) em cima do decode JSON. O engine sozinho (~0.7 µs / 12 allocs / 160 B) explica a maior parte; o resto é wiring (`Validator()`, montagem 422, etc.).

GET sem body **não** é impactado pela feature de validation (mesmo stack fasthttp ~450–650 ns).

### Memória (MemStats, 50k ops após warmup) — pós quick wins RAM

| Operação | B/op (TotalAlloc) | allocs/op (Mallocs) |
|----------|-------------------|---------------------|
| `ValidateStruct` válido | **~48** (era 160) | **1.0** (era 12) |
| HTTP `BodyJSON` | ~2530 (era 2596) | **34** (era 44) |
| HTTP `Bind` only | ~2184 | 22 |

**Quick wins (2026-07-18):** `normalizeRuleName` zero-alloc; skip `reflect.New` se tipo sem `Validate()`; `in`/`oneof` sem `strings.Split`.

Delta BodyJSON − Bind ≈ **+345 B / +12 allocs** (antes +410 B / +22) — custo real da validação no POST.

### Onde ainda dá para cortar

| Alvo | Ideia |
|------|--------|
| `applyRule` | Evitar `Interface()` boxing; passar `reflect.Value` aos builtins |
| Lookup | Pré-bind `ValidatorFunc` no type-meta (sem string no hot path) |
| HTTP 422 | Serializar `fields` sem `map[string]string` intermediário |
| JSON | Codegen / `Write` pré-serializado (já no path estático) |

## Arquivos

- `bench/rps_light_test.go` — **RPS leve** (`TestRPSLight`: GOMAXPROCS=2, 1.5s, conc 1/2/4 + in-process)
- `bench/bench_test.go` — Frameworks / Concurrent / InProcess / Throughput  
- `bench/comprehensive_test.go` — alloc scenarios, middleware overhead, routing scale
- `bench/validation_bench_test.go` — Bind vs BodyJSON + engine + MemStats
- `usecases/validator/validate_test.go` — `BenchmarkValidateStruct`
