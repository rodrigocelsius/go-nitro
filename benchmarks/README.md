# Benchmarks

Reproducible HTTP comparison of Nitro against fasthttp, Fiber, Gin, Echo, chi and net/http
(`/ping`, `/users/:id` JSON, `POST /echo` JSON bind+render).

```bash
cd benchmarks && ./run.sh 5s 64      # Linux, needs taskset
```

Rules of the harness: server and load generator are pinned to separate CPUs, same machine,
same payloads, no logging. Variants:

| name | what it is |
|------|-----------|
| `fasthttp` | raw fasthttp, the ceiling |
| `nitro-bare` | Nitro core: security headers and request-id **disabled** |
| `nitro` | `New()` as shipped (security headers + request-id on) |
| `nitro-defaults` | `nitro.New()` stack (adds Recover, Logger, MaxBody) |
| `fiber` | Fiber v3, no middleware (fair vs `nitro-bare`) |

In-process micro-benchmarks (no network): `go test -bench Perf -benchmem ./adapters/fasthttp`.

Numbers depend on the machine; compare variants within one run, never across machines.
