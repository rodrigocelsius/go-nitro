# Benchmarks

**Language:** [English](benchmarks.md) · [Português](../pt/extra/benchmarks.md)

Nitro vs Fiber vs `net/http` (stdlib).

## Environment (latest run)

- **CPU**: AMD Ryzen 7 8700G
- **OS**: Linux amd64
- **Go**: 1.24.0 (toolchain)
- **Nitro**: defaults off in the bench (`DisableDefaults`, no RequestID/headers) for a fair comparison
- **Command**:

```bash
cd bench
go test -run='^$' -bench='Benchmark(Nitro|Fiber|Stdlib)InProcess|BenchmarkFrameworks|BenchmarkConcurrent' -benchmem -benchtime=300ms -count=1 .

# Light RPS (GOMAXPROCS=2, 1.5s, conc 1/2/4) — ~15–20s total:
go test -run='^TestRPSLight$' -v -count=1 .
```

## RPS Light (2026-07-22) — `TestRPSLight`

Constrained hardware: `GOMAXPROCS=2`, **1.5s** window, concurrency **1 / 2 / 4**.

### Real HTTP

| Conc | stdlib RPS | Nitro RPS | Fiber RPS | Nitro p99 |
|------|------------|-----------|-----------|-----------|
| 1 | ~34k | **~36k** | ~36k | ~69 µs |
| 2 | ~84k | **~97k** | ~95k | ~76 µs |
| 4 | ~83k | **~97k** | ~95k | ~174 µs |

### In-process (fasthttp Handler, no TCP)

| Framework | RPS |
|-----------|-----|
| Nitro | ~11.3M |
| Fiber | ~13.8M |

**Takeaway:** on TCP with 2 cores, Nitro and Fiber tie (~36k→97k RPS). In-process Fiber usually wins (fewer layers on the raw handler); use real HTTP for product decisions.

## Results (2026-07-22) — ns/op benches

### Real HTTP (client → server TCP)

| Route | stdlib | Nitro | Fiber |
|------|--------|-------|-------|
| `GET /` | 32.0 µs · 78 alloc | **29.1 µs** · 56 alloc | 29.8 µs · 57 alloc |
| `GET /hello/:name` | 35.9 µs · 81 | **30.1 µs** · 56 | 30.1 µs · 58 |
| `GET /health` | 32.0 µs · 78 | 30.0 µs · 56 | **29.6 µs** · 57 |
| `GET /ok` (JSON) | 32.4 µs · 79 | **30.1 µs** · 58 | 30.1 µs · 59 |

**Takeaway:** Nitro ≈ Fiber (±3%), both ~8–15% faster than stdlib.

### Parallel (`b.RunParallel`)

| Framework | ns/op | B/op | allocs |
|-----------|-------|------|--------|
| stdlib | ~3.7k | ~6.5k | 72 |
| Nitro | ~2.7k | ~4.0k | 51 |
| **Fiber** | **~2.6k** | ~4.1k | 52 |

**Takeaway:** technical tie (Fiber ~5% ahead in this run).

### In-process (no TCP)

| Scenario | Nitro | stdlib | Fiber (`app.Test`)* |
|---------|-------|--------|---------------------|
| `GET /` | **491 ns** · 9 alloc | 1740 ns · 18 | ~5.1 µs · 30 |
| `GET /hello` | 633 ns · 9 | 2022 ns · 21 | ~5.2 µs · 31 |
| `GET /ok` | 672 ns · 11 | 1924 ns · 19 | ~5.2 µs · 32 |

\*Fiber `app.Test` is not a pure fasthttp hot path — prefer real HTTP / parallel.

## Results (2026-07-17)

### Real HTTP (client → server TCP)

| Route | stdlib | Nitro | Fiber |
|------|--------|-------|-------|
| `GET /` | 32.2 µs · 69 alloc | **28.2 µs** · 47 alloc | **27.5 µs** · 48 alloc |
| `GET /hello/:name` | 31.9 µs · 72 | 28.1 µs · 47 | 29.4 µs · 49 |
| `GET /health` | 31.2 µs · 69 | 28.0 µs · 47 | 28.4 µs · 48 |
| `GET /ok` (JSON) | 32.2 µs · 70 | 28.6 µs · 49 | 28.3 µs · 50 |

**Takeaway:** Nitro ≈ Fiber (±3%), both ~10–15% faster than stdlib and with ~30% fewer allocs on the HTTP path.

### Parallel (`b.RunParallel`)

| Framework | ns/op | B/op | allocs |
|-----------|-------|------|--------|
| stdlib | ~3.3k | ~5.6k | 63 |
| **Nitro** | **~2.4k** | **~3.2k** | **42** |
| Fiber | ~2.4k | ~3.2k | 43 |

**Takeaway:** Nitro ≈ Fiber in parallel (technical tie). The old “Fiber ~2×” was a measurement artifact (one shared `http.Client`/`Transport` across targets + no warmup). Now: dedicated client per framework, warmed pool, high `MaxIdleConnsPerHost`.

Also on the server: `Serve()` uses the full `serverConfig()` (`Concurrency: 256*1024` like Fiber/fasthttp, `NoDefaultDate`/`NoDefaultServerHeader`, one recover+pool defer). Earlier benches built a raw `fasthttp.Server{Handler: app.Handler()}`.

### In-process (no TCP)

| Scenario | Nitro | stdlib (`httptest`) | Fiber (`app.Test`) |
|---------|-------|-------------------|--------------------|
| `GET /` | **492 ns** · 9 alloc | 1757 ns · 17 | ~6.0 µs · 30* |
| `GET /hello` | 627 ns · 9 | 2045 ns · 20 | ~6.0 µs · 31* |
| `GET /ok` | 704 ns · 11 | 2001 ns · 18 | ~6.2 µs · 32* |

\*Fiber `app.Test` goes through a `net/http` adapter — not the pure fasthttp hot path; use real HTTP / parallel to compare Fiber fairly.

### Historical throughput (RPS, 5s, `TestThroughput`)

| Concurrency | stdlib | Nitro | Fiber |
|-------------|--------|-------|-------|
| 1 | ~33k | ~35k | ~36k |
| 10 | ~120k | **~196k** | ~179k |
| 50 | ~256k | ~385k | **~397k** |
| 100 | ~325k | ~414k | **~421k** |

## What was already optimized (history)

- Server timeouts + `DisableHeaderNamesNormalizing` + concurrency
- `b2s` no hot path; pool de `FastCtx`
- Recover + `pool.Put` in a single defer
- **(2026-07-17)** `SetContentTypeBytes` em `Write` / `JSON` / `Text`
- **(2026-07-17)** `Concurrency: 256*1024` (antes `GOMAXPROCS*256`), `Serve()` / `FastHTTPServer()`, `NoDefault*`
- **(2026-07-17)** Bench paralelo justo (client por alvo + warmup)
- **(2026-07-22)** `Write`/`Bytes` via `SetBodyRaw` (como Fiber `Send`); `ResetHot`; `handlerHot` + `DisableInlineRecover`; benches com `GetDomain`

## What can still improve (prioritized)

1. **Parallelism vs Fiber** — fixed in the measurement; remaining engine gap is noise (±5%).
2. **Query** — `Int`/`Peek` already low-alloc; Float/Bool/IntE/QuerySlice optimized. Remaining allocs come from fasthttp URI parse on a cold bench.
3. **JSON** — `AppendJSON` + `SetBodyRaw`; stdlib path uses buffer pool + `Encoder`. Prefer codegen/`Write` on the hot path.
4. **Logger Defaults** — ~~4561 ns~~ → **~2800 ns** (append + direct stderr, skip health/metrics, no double wrap). Opt-in: `Async: true`.

## Fair methodology

```go
nitro.New(nitro.Config{
    DisableDefaults:      true,
    DisableRequestID:     true,
    DisableTelemetry:     true,
    DisableInlineRecover: true,
    Security:             nitro.SecurityConfig{DisableHeaders: true},
})
// Prefer app.Serve(ln) — not a raw fasthttp.Server{Handler: app.Handler()}.
// Static hot path: GetDomain + c.Write (SetBodyRaw).
```

Shared HTTP servers via `sync.Once`. Parallel: one `http.Client` per framework + pool warmup.

## Validation (`BenchmarkValidateStruct` + HTTP)

### Run

```bash
# Isolated engine
cd usecases/validator && go test -bench=BenchmarkValidateStruct -benchmem -count=3

# HTTP in-process + MemStats
cd bench
go test -run=TestValidationMemorySample -v -count=1
go test -run='^$' -bench='BenchmarkValidation' -benchmem -benchtime=500ms -count=3
```

File: `bench/validation_bench_test.go`.

### Isolated engine (no HTTP/JSON) — AMD Ryzen 7 8700G, 2026-07-18

| Case | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| Pre type-meta cache (Phase 8 baseline) | ~1470 | 696 | 29 |
| **Valid (with cache + RAM wins)** | **~470** | **48** | **1** |
| Invalid (collects all errors) | ~960 | 328 | 10 |
| Invalid + `StopOnFirstError` | ~360 | 142 | 6 |

### HTTP in-process: Bind vs BodyJSON (validation cost per request)

Same valid JSON `{"name":"John","email":"john@example.com","status":"active"}`.

| Path | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| `POST /bind` (`c.Bind` only) | **~1700** | 2184 | 22 |
| `POST /validate` (`c.BodyJSON`) | **~2950** | 2595 | 44 |
| **Delta = validation** | **~+1250 (~+73%)** | **~+410** | **~+22** |
| Invalid `BodyJSON` → 422 | ~4570 | 4095 | 69 |
| `BodyJSON` nested | ~3670 | 2732 | 50 |
| `GET /ping` (control, no body) | ~510 | 1634 | 9 |

**Takeaway:** on the HTTP hot path, validation adds ~1.2 µs and ~22 allocs (~410 B) on top of JSON decode. The engine alone (~0.7 µs / 12 allocs / 160 B) explains most of it; the rest is wiring (`Validator()`, 422 assembly, etc.).

GET with no body is **not** impacted by the validation feature (same fasthttp stack ~450–650 ns).

### Memory (MemStats, 50k ops after warmup) — after RAM quick wins

| Operation | B/op (TotalAlloc) | allocs/op (Mallocs) |
|----------|-------------------|---------------------|
| Valid `ValidateStruct` | **~48** (was 160) | **1.0** (was 12) |
| HTTP `BodyJSON` | ~2530 (was 2596) | **34** (was 44) |
| HTTP `Bind` only | ~2184 | 22 |

**Quick wins (2026-07-18):** `normalizeRuleName` zero-alloc; skip `reflect.New` se tipo sem `Validate()`; `in`/`oneof` sem `strings.Split`.

Delta BodyJSON − Bind ≈ **+345 B / +12 allocs** (was +410 B / +22) — real validation cost on POST.

### Where we can still cut

| Target | Idea |
|------|--------|
| `applyRule` | Avoid `Interface()` boxing; pass `reflect.Value` to builtins |
| Lookup | Pre-bind `ValidatorFunc` on type-meta (no string on the hot path) |
| HTTP 422 | Serialize `fields` without an intermediate `map[string]string` |
| JSON | Codegen / pre-serialized `Write` (already on the static path) |

## Files

- `bench/rps_light_test.go` — **light RPS** (`TestRPSLight`: GOMAXPROCS=2, 1.5s, conc 1/2/4 + in-process)
- `bench/bench_test.go` — Frameworks / Concurrent / InProcess / Throughput  
- `bench/comprehensive_test.go` — alloc scenarios, middleware overhead, routing scale
- `bench/validation_bench_test.go` — Bind vs BodyJSON + engine + MemStats
- `usecases/validator/validate_test.go` — `BenchmarkValidateStruct`
