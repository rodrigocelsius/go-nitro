# Advanced security audit — Nitro v1.2

**Language:** [English](AUDIT.md) · [Português](../pt/security/AUDIT.md)

| Field | Value |
|-------|--------|
| **Date** | 2026-08-10 |
| **Scope** | Nitro framework (`pkg/`, `adapters/`, `usecases/`, `domain/`, `data/`, `blinq/`, `cmd/nitrogen`) |
| **Out of scope** | Third-party apps; demos only when they illustrate insecure defaults |
| **Version analyzed** | CLI/banner **1.2.0** (`main` branch) |
| **Fixes applied** | **1.2.1** — see [8. Fix status](#8-fix-status-121) |
| **Classification** | Internal findings — do **not** open a public issue with an exploit PoC |

Complementary report to the policy in [`SECURITY.md`](../SECURITY.md).  
To report vulnerabilities: `security@nitro.dev` (or GitHub private vulnerability reporting).

---

## 1. Methodology and tests run

### 1.1 Manual code review (threat modeling)

Areas walked with direct code reading:

| Area | Main files | Vectors assessed |
|------|-------------------|-------------------|
| Headers / defaults | `adapters/fasthttp/security.go`, `pkg/defaults.go` | CSP, HSTS, nosniff, frame deny |
| CORS | `pkg/defaults.go`, security CORS | `*` + credentials, origins |
| CSRF | `csrf.go`, `bff_web.go` | Double-submit, RNG, timing |
| Web/Mobile BFF | `bff_web.go`, `bff_mobile.go`, `bff.go` | Token leak, cookies |
| JWT / OAuth / OIDC | `adapters/auth/jwt`, `oauth.go`, OIDC | `alg`, empty secret, OAuth state CSRF, revoke |
| Body / upload | `limits.go`, `context.go`, `server.go` | Memory DoS, multipart |
| Static / SPA | `static.go` | Path traversal, prefix bypass |
| Rate limit | `ratelimit.go` | IP spoof via XFF |
| SQL / Resource | `blinq/.../dialect`, `store_repo.go`, `resource_query.go` | Identifier injection / `?sort=` |
| Cookies / Redirect / IP | `context.go` | Secure flag, open redirect, ClientIP |
| CLI | `cmd/nitrogen/install.go`, `project.go` | Predictable secrets in compose |

### 1.2 Automated tools

| Tool | Target | Summary result |
|------------|------|--------------------|
| **govulncheck** | `adapters/fasthttp`, `pkg` | Dozens of CVEs in **Go 1.24.2 stdlib** and transitive deps (`golang.org/x/net`, `x/text`, etc.). Main mitigation: **update the Go toolchain** (≥ 1.24.4 / ideally the project’s 1.25.x LTS) and bump `x/net` / `x/text`. |
| **gosec** | `adapters/fasthttp` | 22 issues (mostly LOW `G104` ignored errors; MEDIUM: SHA1 in WebSocket protocol — expected by RFC6455; `WriteFile` perms; static path; HIGH `G115` overflows in ws/dashboard — low practical impact). |
| **gosec** | `adapters/auth/jwt`, `blinq/provider/sql` | 0 automated issues (does not replace logic review). |
| **nitrogen security** | HTTP runtime | Available (`headers` / `csrf` / `xss` / `all`) — useful against a running app; not executed here without a target server. |

Reference commands:

```bash
# Vulnerable dependencies
cd adapters/fasthttp && govulncheck ./...
cd pkg && govulncheck ./...

# Static analysis
cd adapters/fasthttp && gosec ./...

# Against a local app
nitrogen security all -url http://localhost:8080
```

### 1.3 Controls that are already correct

| Control | Evidence |
|----------|-----------|
| Default security headers | CSP `default-src 'self'`, `X-Content-Type-Options`, `X-Frame-Options: DENY`, HSTS in production |
| CORS | Rejects `AllowCredentials` + `*`; wide CORS only in `EnvDevelopment` |
| JWT HMAC | Rejects method ≠ HMAC (mitigates `alg: none` / algorithm confusion) |
| OIDC | RSA signature via JWKS |
| Panic recover (hot path) | Generic JSON response, no stack to the client |
| Max body | Default **1 MiB** (Content-Length + real body) |
| Rate limit key | Default uses `RemoteIP` (not blind XFF) |
| Static | Blocks `..` / `%2e` + `filepath.Abs` |
| Standalone CSRF | `crypto/rand` + `subtle.ConstantTimeCompare` |
| Access log | Does not log Authorization / body / cookies |
| bcrypt | `DefaultCost` + `CompareHashAndPassword` |
| Password in JSON | `User.PasswordHash` with `json:"-"` |

---

## 2. Executive summary

| Severity | Count | Recommended action |
|------------|-----|------------------|
| **Critical** | 3 | Fix before any production use of Web BFF / OAuth helpers |
| **High** | 4 | Fix in the next patch release |
| **Medium** | 6 | Plan + document footguns |
| **Low / Info** | 6 | Hardening and DX |

**Top 5 (priority):**

1. Bearer/`X-User-ID` injected into the **response** by the BFF (`SetHeader` = response).
2. Web BFF CSRF with a **deterministic** (not random) token.
3. OAuth `state` only in request memory (OAuth CSRF broken).
4. Malformed `Secure=%v` cookie on the BFF.
5. `QuoteIdent` / `?sort=` without a required whitelist → SQL risk via identifiers.

---

## 3. Detailed findings

### C-01 — JWT leaked in the HTTP response (Web/Mobile BFF)

| | |
|--|--|
| **Severity** | Critical |
| **Location** | `adapters/fasthttp/bff_web.go` (~101–104), `bff_mobile.go` (~100–103), `context.go` `SetHeader` (~356–358) |
| **Test** | Code reading: `SetHeader` writes `Response.Header`. BFF middleware calls `fc.SetHeader("Authorization", "Bearer "+…)` and `X-User-ID`. |

**Scenario:** After BFF authentication, the browser / client receives the access token in **response** headers. XSS, a malicious extension, or DevTools expose the session. On Web BFF, the proxy reads Authorization from the **request** — the cookie is not automatically resent upstream → leak **and** functional failure.

**Fix:**

```go
// Inject into the REQUEST, never the response:
req := fc.FasthttpCtx().Request
req.Header.Set("Authorization", "Bearer "+accessToken)
req.Header.Set("X-User-ID", claims.UserID)
```

Or pass headers only on the `proxyRequest` / `extractHeaders` path. Remove any mirroring of Bearer on the response.

---

### C-02 — Web BFF CSRF: predictable token

| | |
|--|--|
| **Severity** | Critical |
| **Location** | `adapters/fasthttp/bff_web.go` `generateCSRFToken` (~301–309) |
| **Test** | Code uses `alphabet[i%62]` — fixed 32-char sequence, no `crypto/rand`. |

**Scenario:** An attacker predicts the CSRF cookie/header and bypasses double-submit on cookie-authenticated POSTs.

**Fix:** Reuse `generateSecureCSRFToken()` from `csrf.go` (`crypto/rand` + hex). Remove the weak function. Compare with `subtle.ConstantTimeCompare` (today the loop in `compareCSRFTokens` is also not truly constant-time).

---

### C-03 — OAuth state does not persist across requests

| | |
|--|--|
| **Severity** | Critical |
| **Location** | `adapters/fasthttp/oauth.go` (~58–76, ~34–39) |
| **Test** | `OAuthRedirectHandler` does `fctx.Set("oauth_state", state)` on the **same** request as the redirect; on the callback the value does not exist. |

**Scenario:** State validation always fails **or** apps “fix” it by removing the check → CSRF on the OAuth flow (forced login into the attacker’s account).

**Fix:** Persist state in an `HttpOnly` + `Secure` + `SameSite=Lax` cookie (signed/HMAC) or Redis/session store; on the callback compare with `subtle.ConstantTimeCompare`. Document that the current helper is **not** production-ready until then.

---

### H-01 — Malformed `Secure` attribute on BFF cookies

| | |
|--|--|
| **Severity** | High |
| **Location** | `bff_web.go` `setSecureCookie` (~227–233) |
| **Test** | `Secure=%v` → `Secure=true` / `Secure=false`. In Set-Cookie, `Secure` is a **flag** (presence = on). |

**Scenario:** With `CookieSecure: false`, the header may still include `Secure=false`, which many clients treat as Secure → cookies break on local HTTP **or** inconsistent browser behavior.

**Fix:** Use `fasthttp.Cookie` / a typed API: only append `Secure` when `cfg.CookieSecure == true`. Same for the CSRF cookie (currently without `Secure`).

---

### H-02 — SQL injection / breakage via identifiers (`QuoteIdent` + sort)

| | |
|--|--|
| **Severity** | High |
| **Location** | `blinq/provider/sql/dialect/postgres.go` `QuoteIdent`; `data/adapter/pgx/store_repo.go` (~90–109); `usecases/resource_query.go` (~37–52) |
| **Test** | `QuoteIdent` = `"`+name+`"` without doubling `"`. If `Sortable` is empty, **any** `?sort=` is accepted (`ok := len(opts.Sortable) == 0`). An unmapped column uses the raw user value. |

**Scenario:** `?sort=id";DROP...` or identifiers with `"` can change the generated SQL (quote break / identifier injection). Filter values use placeholders (OK); the risk is in column/order **names**.

**Fix:**

1. Validate identifiers: `^[a-zA-Z_][a-zA-Z0-9_]*$` **or** escape `"` → `""`.
2. Empty `Sortable` / `Filterable` = **disable** sort/filter (do not “allow everything”).
3. Never use `f.Field` / `s.Field` unless it is on the `jsonToDB` whitelist.

---

### H-03 — JWT accepts an empty secret

| | |
|--|--|
| **Severity** | High |
| **Location** | `adapters/auth/jwt/jwt.go` `New` (~22–28) |
| **Test** | `New` does not validate `cfg.JWTSecret`; HS256 with key `""` is trivial. |

**Fix:** Fail at boot if `len(secret) < 32` (or require documented entropy). Expose a clear error in the CLI/`NewAuth`.

---

### H-04 — Standalone CSRF cookies without `Secure`

| | |
|--|--|
| **Severity** | High (on mixed HTTPS) |
| **Location** | `csrf.go` (~121–126); BFF CSRF cookie (~255–256) |
| **Test** | CSRF cookie: Path + SameSite, no Secure flag. |

**Fix:** Emit `Secure` when TLS / `EnvProduction`. Keep the CSRF cookie readable by JS (no HttpOnly) in the double-submit pattern — that is expected.

---

### M-01 — JWT refresh without rotation/revocation; logout is a no-op

| | |
|--|--|
| **Severity** | Medium |
| **Location** | `jwt.go` `RefreshToken`, `RevokeToken` (~90–148) |

**Scenario:** A stolen refresh stays valid until `exp`. `RevokeToken` returns `nil`. A new access after refresh drops email/roles (only `sub`).

**Fix:** JTI / refresh-family store; blacklist on logout; re-query the user on refresh.

---

### M-02 — Residual path traversal (`HasPrefix` on static)

| | |
|--|--|
| **Severity** | Medium |
| **Location** | `static.go` (~101–107) |

**Classic scenario:** `absRoot=/var/www` and path `/var/www2/...` pass `strings.HasPrefix`.

**Fix:**

```go
rel, err := filepath.Rel(absRoot, absPath)
if err != nil || strings.HasPrefix(rel, "..") {
    // forbid
}
```

---

### M-03 — Open redirect in the `Redirect` API

| | |
|--|--|
| **Severity** | Medium (app footgun) |
| **Location** | `context.go` `Redirect` (~664–672) |

**Scenario:** App does `c.Redirect(c.Query("next"))` → `//evil.com`.

**Fix:** `SafeRedirect` helper (only relative paths starting with `/` and without `//`, or a host allowlist). Document the risk in the guide.

---

### M-04 — `ClientIP` trusts XFF without proxy trust

| | |
|--|--|
| **Severity** | Medium |
| **Location** | `context.go` `ClientIP` (~675–688) |

**Note:** Default rate limit uses `RemoteIP` (OK). Anyone using `ClientIP` for auth/quota suffers spoofing.

**Fix:** Same model as `TrustedProxyKeyFunc`; documented default = `RemoteIP`.

---

### M-05 — `PublicStack` = CORS `*` in production

| | |
|--|--|
| **Severity** | Medium (footgun) |
| **Location** | `pkg/defaults.go` |

**Fix:** In prod, warn at boot if origins are `*`; require an explicit list for APIs with sensitive data.

---

### M-06 — Multipart upload / `SaveFile`

| | |
|--|--|
| **Severity** | Medium |
| **Location** | `context.go` FormFile/SaveFile; server MaxRequestBodySize |

**Scenario:** Entire file in memory; `SaveFile` with an app-controlled path can write outside the intended directory if the app concatenates user names.

**Fix:** Per-file limit; `MaxRequestBodySize` aligned with Nitro max body; sanitize basename; `0600` permissions (gosec G306).

---

### L-01 — `InternalError` returns `err.Error()` to the client

| | |
|--|--|
| **Severity** | Low |
| **Location** | `context.go` (~1208–1213) |

**Fix:** Always a generic message to the client; log the real error on the server.

---

### L-02 — Request-ID accepted from the client without sanitization

| | |
|--|--|
| **Severity** | Low |
| **Location** | `requestid.go` |

**Fix:** Validate charset/length or prefix a generated ID + the external value.

---

### L-03 — Misleading `RecoverMiddleware`

| | |
|--|--|
| **Severity** | Low |
| **Location** | `recover.go`, `defaults.go` |

The real recover is in the server’s inline handler. The middleware does not `defer recover()`.

**Fix:** Unify or document clearly.

---

### L-04 — CLI `nitrogen install`: predictable DB passwords and ports

| | |
|--|--|
| **Severity** | Low (dev) / Medium if compose is production |
| **Location** | `cmd/nitrogen/install.go` |

**Fix:** Generate a random password; bind `127.0.0.1`; “dev only” warning in compose; `.env.example` without a real secret.

---

### I-01 — Rate limit is opt-in

Intentional (`SECURITY.md`). APIs under abuse should use `APIStack` / `RateLimit`.

---

### I-02 — WebSocket SHA1 (gosec G401/G505)

SHA1 in the WebSocket handshake is **required by RFC 6455** — do not treat it as an app crypto vulnerability. Keep documented nosec.

---

## 4. Dependencies (govulncheck)

Summary (August 2026):

- Toolchain reported in modules: **Go 1.22.2 / 1.24.x** — several stdlib fixes require **≥ 1.24.4** (ideally align the monorepo to a recent supported version).
- Transitive deps: update `golang.org/x/net`, `golang.org/x/text` to versions with the fix (see `govulncheck` output).
- Many findings are indirect “reachable via call graph” (TLS, multipart, URL) — prioritize a Go bump + controlled `go get -u` + re-scan.

Checklist:

```text
[ ] go version aligned (min. patch with crypto/tls, net/url, x509 fixes)
[ ] go get golang.org/x/net@latest / x/text@latest in affected go.mod files
[ ] govulncheck ./... clean (or only documented accepted vulns)
```

---

## 5. Suggested fix plan

### Sprint A — Critical (BFF / OAuth)

1. Fix header injection on the **request** (C-01).
2. BFF CSRF with `crypto/rand` + constant-time (C-02).
3. OAuth state in cookie/store (C-03).
4. Cookies via typed `Secure`/`SameSite`/`HttpOnly` API (H-01, H-04).

### Sprint B — High (JWT / SQL)

5. Validate `JWTSecret` in `New` (H-03).
6. Required whitelist + safe `QuoteIdent` (H-02).
7. Refresh revocation / JTI (M-01).

### Sprint C — Hardening

8. Static `filepath.Rel` (M-02).
9. `SafeRedirect` + docs (M-03).
10. `ClientIP` + trusted proxies (M-04).
11. Warn `PublicStack` in prod (M-05).
12. Upload limits / SaveFile (M-06).
13. Generic `InternalError` (L-01).
14. CLI random passwords (L-04).
15. Bump Go + deps (section 4).

### Regression tests to add

| Test | Goal |
|-------|----------|
| Unit: `generateCSRFToken` | Entropy / non-determinism |
| Unit: BFF middleware | `Authorization` does **not** appear in `Response.Header` |
| Unit: OAuth callback | Valid state via cookie; invalid state → 400 |
| Unit: `QuoteIdent` / List sort | Reject `sort` outside the whitelist and special chars |
| Unit: `New(JWTSecret:"")` | Must fail |
| Unit: Static | Path outside root (`/var/www` vs `/var/www2`) |
| Integration: `nitrogen security all` | Headers + CSRF + XSS on `auth` / BFF examples |

---

## 6. Surface matrix

| Surface | Post-audit status |
|------------|----------------------|
| Security headers | OK |
| Default CORS | OK (+ PublicStack footgun) |
| Standalone CSRF | OK |
| BFF CSRF / cookies | **Fixed in 1.2.1** |
| JWT alg confusion | OK |
| JWT secret / revoke | **Fixed in 1.2.1** |
| OAuth helpers | **Fixed in 1.2.1** |
| Body limit | OK |
| Multipart | Partial (0600 perms) |
| Static | **Fixed in 1.2.1** (`filepath.Rel`) |
| Rate limit | OK when enabled |
| SQL values (placeholders) | OK |
| SQL identifiers / Resource sort | **Fixed in 1.2.1** |
| Open redirect API | **Mitigated** (`SafeRedirect`) |
| CLI scaffold | **Improved** (random password + bind 127.0.0.1) |

---

## 7. Internal references

- Policy: [`SECURITY.md`](../SECURITY.md)
- Defaults: `pkg/defaults.go`
- CLI security probes: `cmd/nitrogen/security.go`
- Auth example: `examples/v1/auth`, `examples/v1/auth-keycloak`

---

## 8. Fix status (1.2.1)

| ID | Status | Notes |
|----|--------|-------|
| C-01 | ✅ | `SetRequestHeader` on Web/Mobile BFF |
| C-02 | ✅ | `generateSecureCSRFToken` + `csrfTokenEqual` |
| C-03 | ✅ | HttpOnly `_oauth_state` cookie |
| H-01 / H-04 | ✅ | `SetCookieWithSameSite`; Secure only as a flag |
| H-02 | ✅ | Sort whitelist; skip unknown cols; escape `QuoteIdent` |
| H-03 | ✅ | `MinSecretLen=16`; `New` returns error / `MustNew` |
| M-01 | ✅ | Refresh rotation + in-memory JTI blacklist |
| M-02 | ✅ | `filepath.Rel` |
| M-03 | ✅ | `SafeRedirect` |
| M-04 | ✅ | `SetTrustedProxies` + `RemoteIP` |
| M-05 | ✅ | stderr warn on `PublicStack` in production |
| M-06 | ✅ partial | `SaveFile` with `0600` |
| L-01 | ✅ | Generic `InternalError` |
| L-02 | ✅ | Sanitized Request-ID |
| L-04 | ✅ | Random password + `127.0.0.1` in compose |
| Go deps | ⏳ | Monorepo toolchain still mixed; prefer Go ≥ 1.24.4 / 1.26 |

Regression tests: `adapters/fasthttp/security_fixes_test.go`, `adapters/auth/jwt/jwt_test.go`.

---

*Document produced from manual review + govulncheck + gosec. Critical/High fixes applied in 1.2.1. Does not replace an external pentest.*
