# Resource security (V-04)

Secure-by-default rules for `app.Resource`. See also [`resource-hooks.md`](resource-hooks.md) and AI playbook § R2b.

## Problem

`app.Resource` does **not** attach auth by itself. An unconfigured Resource exposes full CRUD (list/get/create/update/delete).

## Required pattern

```go
auth := nitro.NewAuth(nitro.AuthConfig{JWTSecret: secret}).WithMemoryStore()
app.AuthRoutes("/auth", auth)
app.SetDefaultResourceMiddleware(nitro.JWTMiddleware(auth.Provider()))

// Protected by default:
app.Resource("/api/catalog/albums", &model.Album{}, stores.Albums, &nitro.ResourceConfig{
    NoCache: true,
    // Public: false (default) → inherits JWT middleware
})

// Intentionally open (rare):
app.Resource("/api/public/ping-items", &model.Ping{}, store, &nitro.ResourceConfig{
    Public: true,
    Only:   []string{nitro.ActionList, nitro.ActionGet},
})
```

## Production guard (V-04)

In `EnvProduction`, registering a Resource with **no** Middleware/Read/Write and **without** `Public: true` **panics** at boot.

Escape hatches (prefer avoiding them):

- `ResourceConfig.Public = true` — acknowledge open CRUD
- `Config.Security.AllowUnauthenticatedResources = true` — warn instead of panic (ops opt-in)

Development: warning on stderr only.

## Ownership / tenant (V-05)

`app.Resource` does **not** scope by owner or tenant by default. Close the IDOR gap:

1. Resolve tenant on every request (after JWT):

```go
app.Use(nitro.JWTMiddleware(auth.Provider()))
app.Use(nitro.TenantMiddleware(nitro.TenantConfig{Require: true}))
```

JWT access tokens include `tenant_id` when `domain.User.TenantID` is set at login.
Header fallback: `X-Tenant-ID` is **off by default** (a client-supplied tenant is an IDOR). Opt in with `AllowHeader: &true` only for trusted internal callers or a gateway that overwrites the header.

2. Scope Resource CRUD — **automatic by convention** when the model has a `tenant_id` column (embed `nitro.TenantModel`, or `nitrogen make:scaffold Order … --tenant`):

```go
app.Resource(&Order{}, store) // list filter, get/update/delete check, create stamp
// opt out: &nitro.ResourceConfig{NoTenantScope: true}
// custom column/source: TenantScope: nitro.TenantFromCtx(), TenantColumn: "org_id"
```

Or use `OwnerCheck` for user-owned rows (Get/Update/Delete only — not List).

## Do / Don't

| Do | Don't |
|----|--------|
| `SetDefaultResourceMiddleware(JWT…)` once after AuthRoutes | `Public: true` on every Resource to silence warnings |
| `tenant_id` column (auto-scope) + `TenantMiddleware` / `OwnerCheck` | Assume list filters alone close IDOR |
| `readonly` / `computed` on privileged & denorm fields | Trust client JSON for `role`, `artist_name`, balances |

## Related

- Mass assignment: `readonly:"true"` / `computed:"true"` — [`models.md`](models.md)
- Business actions: Commands — [`business-resource.md`](business-resource.md)
- Audit notes: `docs/security/AUDIT.md`, vuln.md V-04 / V-05
