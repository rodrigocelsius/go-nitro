# Performance notes

Reproduce with `benchmarks/run.sh` (see [`benchmarks/README.md`](../../benchmarks/README.md)); in-process: `go test -bench Perf -benchmem ./adapters/fasthttp`.

- **Core is Fiber-class.** With request-id and security headers disabled (`DisableRequestID`, `Security.DisableHeaders`) a request costs ~265 ns and 0 allocations in-process.
- **Defaults are not free.** The 8 security headers cost ~0.4 µs per request (fasthttp header API) and the request id ~0.1 µs. Keep them in production; disable only for internal benchmarks.
- **Access log:** the default logger writes synchronously (one `write` syscall per request). For high traffic use `LoggerConfig{Async: true}` — lines are batched into a single write (up to 64 KiB); `nitroadapter.LoggerDropped()` reports lines dropped under back-pressure.
- **JSON binding** skips the second "presence" decode unless the type declares `default:` tags.
- Never retain `*nitro.Ctx` after the handler: it is pooled (`c.Detach()` for async work).
