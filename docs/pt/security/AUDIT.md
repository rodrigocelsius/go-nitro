# Auditoria de segurança avançada — Nitro v1.2

**Idioma:** [Português](AUDIT.md) · [English](../../security/AUDIT.md)

| Campo | Valor |
|-------|--------|
| **Data** | 2026-08-10 |
| **Escopo** | Framework Nitro (`pkg/`, `adapters/`, `usecases/`, `domain/`, `data/`, `blinq/`, `cmd/nitrogen`) |
| **Fora de escopo** | Apps de terceiros; demos só quando ilustram defaults inseguros |
| **Versão analisada** | CLI/banner **1.2.0** (branch `main`) |
| **Correções aplicadas** | **1.2.1** — ver seção [8. Status das correções](#8-status-das-correções) |
| **Classificação** | Achados internos — **não** abrir issue pública com PoC de exploração |

Relatório complementar à política em [`SECURITY.md`](../SECURITY.md).  
Para reportar vulnerabilidades: `security@nitro.dev` (ou GitHub private vulnerability reporting).

---

## 1. Metodologia e testes executados

### 1.1 Revisão manual de código (threat modeling)

Áreas percorridas com leitura direta do código:

| Área | Arquivos principais | Vetores avaliados |
|------|---------------------|-------------------|
| Headers / defaults | `adapters/fasthttp/security.go`, `pkg/defaults.go` | CSP, HSTS, nosniff, frame deny |
| CORS | `pkg/defaults.go`, security CORS | `*` + credentials, origins |
| CSRF | `csrf.go`, `bff_web.go` | Double-submit, RNG, timing |
| BFF Web/Mobile | `bff_web.go`, `bff_mobile.go`, `bff.go` | Leak de token, cookies |
| JWT / OAuth / OIDC | `adapters/auth/jwt`, `oauth.go`, OIDC | `alg`, secret vazio, state CSRF, revoke |
| Body / upload | `limits.go`, `context.go`, `server.go` | DoS memória, multipart |
| Static / SPA | `static.go` | Path traversal, prefix bypass |
| Rate limit | `ratelimit.go` | Spoof de IP via XFF |
| SQL / Resource | `blinq/.../dialect`, `store_repo.go`, `resource_query.go` | Injection em identificadores / `?sort=` |
| Cookies / Redirect / IP | `context.go` | Secure flag, open redirect, ClientIP |
| CLI | `cmd/nitrogen/install.go`, `project.go` | Secrets previsíveis em compose |

### 1.2 Ferramentas automatizadas

| Ferramenta | Alvo | Resultado resumido |
|------------|------|--------------------|
| **govulncheck** | `adapters/fasthttp`, `pkg` | Dezenas de CVEs em **stdlib Go 1.24.2** e deps transitivas (`golang.org/x/net`, `x/text`, etc.). Mitigação principal: **atualizar toolchain Go** (≥ 1.24.4 / ideal 1.25.x LTS do projeto) e bump de `x/net` / `x/text`. |
| **gosec** | `adapters/fasthttp` | 22 issues (maioria LOW `G104` erros ignorados; MEDIUM: SHA1 em WebSocket protocol — esperado RFC6455; `WriteFile` perms; static path; HIGH `G115` overflows em ws/dashboard — baixo impacto prático). |
| **gosec** | `adapters/auth/jwt`, `blinq/provider/sql` | 0 issues automatizados (não substitui revisão de lógica). |
| **nitrogen security** | Runtime HTTP | Disponível (`headers` / `csrf` / `xss` / `all`) — útil contra app rodando; não executado aqui sem servidor alvo. |

Comandos de referência:

```bash
# Dependências vulneráveis
cd adapters/fasthttp && govulncheck ./...
cd pkg && govulncheck ./...

# Análise estática
cd adapters/fasthttp && gosec ./...

# Contra app local
nitrogen security all -url http://localhost:8080
```

### 1.3 Controles que já estão corretos

| Controle | Evidência |
|----------|-----------|
| Security headers default | CSP `default-src 'self'`, `X-Content-Type-Options`, `X-Frame-Options: DENY`, HSTS em produção |
| CORS | Rejeita `AllowCredentials` + `*`; CORS amplo só em `EnvDevelopment` |
| JWT HMAC | Rejeita método ≠ HMAC (mitiga `alg: none` / confusão de algoritmo) |
| OIDC | Assinatura RSA via JWKS |
| Panic recover (hot path) | Resposta JSON genérica, sem stack ao cliente |
| Max body | Default **1 MiB** (Content-Length + body real) |
| Rate limit key | Default usa `RemoteIP` (não XFF cego) |
| Static | Bloqueio de `..` / `%2e` + `filepath.Abs` |
| CSRF standalone | `crypto/rand` + `subtle.ConstantTimeCompare` |
| Access log | Não loga Authorization / body / cookies |
| bcrypt | `DefaultCost` + `CompareHashAndPassword` |
| Password no JSON | `User.PasswordHash` com `json:"-"` |

---

## 2. Resumo executivo

| Severidade | Qtd | Ação recomendada |
|------------|-----|------------------|
| **Critical** | 3 | Corrigir antes de qualquer uso de BFF Web / OAuth helpers em produção |
| **High** | 4 | Corrigir na próxima release patch |
| **Medium** | 6 | Planejar + documentar footguns |
| **Low / Info** | 6 | Hardening e DX |

**Top 5 (prioridade):**

1. Bearer/`X-User-ID` injetados na **response** pelo BFF (`SetHeader` = response).
2. CSRF do Web BFF com token **determinístico** (não aleatório).
3. OAuth `state` só na memória da request (CSRF OAuth quebrado).
4. Cookie `Secure=%v` malformado no BFF.
5. `QuoteIdent` / `?sort=` sem whitelist obrigatória → risco de SQL via identificadores.

---

## 3. Achados detalhados

### C-01 — Token JWT vazado na resposta HTTP (BFF Web/Mobile)

| | |
|--|--|
| **Severidade** | Critical |
| **Local** | `adapters/fasthttp/bff_web.go` (~101–104), `bff_mobile.go` (~100–103), `context.go` `SetHeader` (~356–358) |
| **Teste** | Leitura de código: `SetHeader` grava em `Response.Header`. Middleware BFF chama `fc.SetHeader("Authorization", "Bearer "+…)` e `X-User-ID`. |

**Cenário:** Após autenticação BFF, o browser / cliente recebe o access token nos headers de **resposta**. XSS, extensão maliciosa ou DevTools expõem a sessão. No Web BFF, o proxy lê Authorization do **request** — o cookie não é reenviado automaticamente ao upstream → vazamento **e** falha funcional.

**Solução:**

```go
// Injetar no REQUEST, nunca na response:
req := fc.FasthttpCtx().Request
req.Header.Set("Authorization", "Bearer "+accessToken)
req.Header.Set("X-User-ID", claims.UserID)
```

Ou passar headers apenas no caminho de `proxyRequest` / `extractHeaders`. Remover qualquer espelhamento de Bearer na resposta.

---

### C-02 — CSRF Web BFF: token previsível

| | |
|--|--|
| **Severidade** | Critical |
| **Local** | `adapters/fasthttp/bff_web.go` `generateCSRFToken` (~301–309) |
| **Teste** | Código usa `alphabet[i%62]` — sequência fixa de 32 chars, sem `crypto/rand`. |

**Cenário:** Atacante prevê o cookie/header CSRF e contorna double-submit em POSTs autenticados por cookie.

**Solução:** Reutilizar `generateSecureCSRFToken()` de `csrf.go` (`crypto/rand` + hex). Remover a função fraca. Comparação com `subtle.ConstantTimeCompare` (hoje o loop em `compareCSRFTokens` também não é constant-time de verdade).

---

### C-03 — OAuth state não persiste entre requests

| | |
|--|--|
| **Severidade** | Critical |
| **Local** | `adapters/fasthttp/oauth.go` (~58–76, ~34–39) |
| **Teste** | `OAuthRedirectHandler` faz `fctx.Set("oauth_state", state)` no contexto da **mesma** request do redirect; no callback o valor não existe. |

**Cenário:** Validação de state falha sempre **ou** apps “consertam” removendo a checagem → CSRF no fluxo OAuth (login forçado na conta do atacante).

**Solução:** Persistir state em cookie `HttpOnly` + `Secure` + `SameSite=Lax` (assinado/HMAC) ou store Redis/sessão; no callback comparar com `subtle.ConstantTimeCompare`. Documentar que o helper atual **não** é production-ready até isso.

---

### H-01 — Atributo `Secure` malformado em cookies BFF

| | |
|--|--|
| **Severidade** | High |
| **Local** | `bff_web.go` `setSecureCookie` (~227–233) |
| **Teste** | `Secure=%v` → `Secure=true` / `Secure=false`. Em Set-Cookie, `Secure` é **flag** (presença = ativo). |

**Cenário:** Com `CookieSecure: false`, o header ainda pode incluir `Secure=false`, que muitos clientes tratam como Secure → cookies quebram em HTTP local **ou** comportamento inconsistente entre browsers.

**Solução:** Usar `fasthttp.Cookie` / API tipada: só acrescentar `Secure` quando `cfg.CookieSecure == true`. Idem para CSRF cookie (hoje sem `Secure`).

---

### H-02 — Injection / quebra de SQL via identificadores (`QuoteIdent` + sort)

| | |
|--|--|
| **Severidade** | High |
| **Local** | `blinq/provider/sql/dialect/postgres.go` `QuoteIdent`; `data/adapter/pgx/store_repo.go` (~90–109); `usecases/resource_query.go` (~37–52) |
| **Teste** | `QuoteIdent` = `"`+name+`"` sem dobrar `"`. Se `Sortable` está vazio, **qualquer** `?sort=` é aceito (`ok := len(opts.Sortable) == 0`). Coluna não mapeada usa o valor cru do usuário. |

**Cenário:** `?sort=id";DROP...` ou identificadores com `"` podem alterar o SQL gerado (quebra de quote / injeção de identificador). Valores de filtro usam placeholders (OK); o risco está em **nomes** de coluna/ordem.

**Solução:**

1. Validar identificadores: `^[a-zA-Z_][a-zA-Z0-9_]*$` **ou** escapar `"` → `""`.
2. `Sortable` / `Filterable` vazios = **desabilitar** sort/filter (não “permitir tudo”).
3. List anti-DoS: `MaxPerPage` / `MaxFilters` / `MaxFilterValueLen` (defaults seguros quando 0).
4. Preferir `RateLimitWith` + `CompositeKeyFunc` / `APIKeyKeyFunc` em APIs M2M.
3. Nunca usar `f.Field` / `s.Field` sem estar no whitelist `jsonToDB`.

---

### H-03 — JWT aceita secret vazio

| | |
|--|--|
| **Severidade** | High |
| **Local** | `adapters/auth/jwt/jwt.go` `New` (~22–28) |
| **Teste** | `New` não valida `cfg.JWTSecret`; HS256 com chave `""` é trivial. |

**Solução:** Falhar no boot se `len(secret) < 32` (ou exigir entropy documentada). Expor erro claro na CLI/`NewAuth`.

---

### H-04 — Cookies CSRF standalone sem `Secure`

| | |
|--|--|
| **Severidade** | High (em HTTPS misto) |
| **Local** | `csrf.go` (~121–126); cookie CSRF BFF (~255–256) |
| **Teste** | Cookie CSRF: Path + SameSite, sem flag Secure. |

**Solução:** Emitir `Secure` quando TLS / `EnvProduction`. Manter cookie CSRF legível por JS (sem HttpOnly) no padrão double-submit — isso é esperado.

---

### M-01 — Refresh JWT sem rotação/revogação; logout no-op

| | |
|--|--|
| **Severidade** | Medium |
| **Local** | `jwt.go` `RefreshToken`, `RevokeToken` (~90–148) |

**Cenário:** Refresh roubado permanece válido até `exp`. `RevokeToken` retorna `nil`. Novo access após refresh perde email/roles (só `sub`).

**Solução:** Store de JTI / família de refresh; blacklist no logout; reconsultar usuário no refresh.

---

### M-02 — Path traversal residual (`HasPrefix` em static)

| | |
|--|--|
| **Severidade** | Medium |
| **Local** | `static.go` (~101–107) |

**Cenário clássico:** `absRoot=/var/www` e path `/var/www2/...` passam em `strings.HasPrefix`.

**Solução:**

```go
rel, err := filepath.Rel(absRoot, absPath)
if err != nil || strings.HasPrefix(rel, "..") {
    // forbid
}
```

---

### M-03 — Open redirect na API `Redirect`

| | |
|--|--|
| **Severidade** | Medium (footgun de app) |
| **Local** | `context.go` `Redirect` (~664–672) |

**Cenário:** App faz `c.Redirect(c.Query("next"))` → `//evil.com`.

**Solução:** Helper `SafeRedirect` (só path relativo começando com `/` e sem `//`, ou allowlist de hosts). Documentar risco no guide.

---

### M-04 — `ClientIP` confia em XFF sem proxy trust

| | |
|--|--|
| **Severidade** | Medium |
| **Local** | `context.go` `ClientIP` (~675–688) |

**Nota:** Rate limit default usa `RemoteIP` (OK). Quem usar `ClientIP` para auth/quota sofre spoof.

**Solução:** Mesmo modelo de `TrustedProxyKeyFunc`; default documentado = `RemoteIP`.

---

### M-05 — `PublicStack` = CORS `*` em produção

| | |
|--|--|
| **Severidade** | Medium (footgun) |
| **Local** | `pkg/defaults.go` |

**Solução:** Em prod, warn no boot se origins `*`; exigir lista explícita para APIs com dados sensíveis.

---

### M-06 — Upload multipart / `SaveFile`

| | |
|--|--|
| **Severidade** | Medium |
| **Local** | `context.go` FormFile/SaveFile; server MaxRequestBodySize |

**Cenário:** Arquivo inteiro em memória; `SaveFile` com path controlado pelo app pode escrever fora do diretório desejado se o app concatenar nomes de usuário.

**Solução:** Limite por arquivo; `MaxRequestBodySize` alinhado ao max body Nitro; sanitizar basename; permissões `0600` (gosec G306).

---

### L-01 — `InternalError` devolve `err.Error()` ao cliente

| | |
|--|--|
| **Severidade** | Low |
| **Local** | `context.go` (~1208–1213) |

**Solução:** Sempre mensagem genérica ao cliente; logar erro real no servidor.

---

### L-02 — Request-ID aceito do cliente sem sanitização

| | |
|--|--|
| **Severidade** | Low |
| **Local** | `requestid.go` |

**Solução:** Validar charset/tamanho ou prefixar ID gerado + valor externo.

---

### L-03 — `RecoverMiddleware` enganoso

| | |
|--|--|
| **Severidade** | Low |
| **Local** | `recover.go`, `defaults.go` |

O recover real está no handler inline do server. O middleware não faz `defer recover()`.

**Solução:** Unificar ou documentar claramente.

---

### L-04 — CLI `nitrogen install`: senhas e portas de DB previsíveis

| | |
|--|--|
| **Severidade** | Low (dev) / Medium se compose for a prod |
| **Local** | `cmd/nitrogen/install.go` |

**Solução:** Gerar senha aleatória; bind `127.0.0.1`; aviso “dev only” no compose; `.env.example` sem secret real.

---

### I-01 — Rate limit opt-in

Intencional (`SECURITY.md`). APIs sob abuso devem usar `APIStack` / `RateLimit`.

---

### I-02 — WebSocket SHA1 (gosec G401/G505)

Uso de SHA1 no handshake WebSocket é **exigido pela RFC 6455** — não tratar como vulnerabilidade de crypto da app. Manter nosec documentado.

---

## 4. Dependências (govulncheck)

Resumo (Agosto/2026):

- Toolchain reportada nos módulos: **Go 1.22.2 / 1.24.x** — várias correções de stdlib exigem **≥ 1.24.4** (ideal alinhar monorepo a uma versão suportada recente).
- Deps transitivas: atualizar `golang.org/x/net`, `golang.org/x/text` para versões com fix (ver saída de `govulncheck`).
- Muitos findings são “reachable via call graph” indireto (TLS, multipart, URL) — priorizar bump de Go + `go get -u` controlado + re-scan.

Checklist:

```text
[ ] go version alinhada (mín. patch com fixes de crypto/tls, net/url, x509)
[ ] go get golang.org/x/net@latest / x/text@latest nos go.mod afetados
[ ] govulncheck ./... limpo (ou só vulns aceitas documentadas)
```

---

## 5. Plano de correção sugerido

### Sprint A — Critical (BFF / OAuth)

1. Corrigir injeção de headers no **request** (C-01).
2. CSRF BFF com `crypto/rand` + constant-time (C-02).
3. State OAuth em cookie/store (C-03).
4. Cookies via API tipada `Secure`/`SameSite`/`HttpOnly` (H-01, H-04).

### Sprint B — High (JWT / SQL)

5. Validar `JWTSecret` no `New` (H-03).
6. Whitelist obrigatória + `QuoteIdent` seguro (H-02).
7. Refresh revocation / JTI (M-01).

### Sprint C — Hardening

8. Static `filepath.Rel` (M-02).
9. `SafeRedirect` + docs (M-03).
10. `ClientIP` + trusted proxies (M-04).
11. Warn `PublicStack` em prod (M-05).
12. Upload limits / SaveFile (M-06).
13. `InternalError` genérico (L-01).
14. CLI senhas aleatórias (L-04).
15. Bump Go + deps (seção 4).

### Testes de regressão a adicionar

| Teste | Objetivo |
|-------|----------|
| Unit: `generateCSRFToken` | Entropia / não-determinismo |
| Unit: BFF middleware | `Authorization` **não** aparece em `Response.Header` |
| Unit: OAuth callback | State válido via cookie; state inválido → 400 |
| Unit: `QuoteIdent` / List sort | Rejeitar `sort` fora do whitelist e chars especiais |
| Unit: `New(JWTSecret:"")` | Deve falhar |
| Unit: Static | Path fora de root (`/var/www` vs `/var/www2`) |
| Integration: `nitrogen security all` | Headers + CSRF + XSS em exemplo `auth` / BFF |

---

## 6. Matriz por superfície

| Superfície | Status pós-auditoria |
|------------|----------------------|
| Security headers | OK |
| CORS default | OK (+ footgun PublicStack) |
| CSRF standalone | OK |
| CSRF / cookies BFF | **Corrigido em 1.2.1** |
| JWT alg confusion | OK |
| JWT secret / revoke | **Corrigido em 1.2.1** |
| OAuth helpers | **Corrigido em 1.2.1** |
| Body limit | OK |
| Multipart | Parcial (perms 0600) |
| Static | **Corrigido em 1.2.1** (`filepath.Rel`) |
| Rate limit | OK quando ligado |
| SQL values (placeholders) | OK |
| SQL identifiers / Resource sort | **Corrigido em 1.2.1** |
| Open redirect API | **Mitigado** (`SafeRedirect`) |
| CLI scaffold | **Melhorado** (senha aleatória + bind 127.0.0.1) |

---

## 7. Referências internas

- Política: [`SECURITY.md`](../SECURITY.md)
- Defaults: `pkg/defaults.go`
- CLI security probes: `cmd/nitrogen/security.go`
- Exemplo auth: `examples/v1/auth`, `examples/v1/auth-keycloak`

---

## 8. Status das correções (1.2.1)

| ID | Status | Notas |
|----|--------|-------|
| C-01 | ✅ | `SetRequestHeader` no BFF Web/Mobile |
| C-02 | ✅ | `generateSecureCSRFToken` + `csrfTokenEqual` |
| C-03 | ✅ | Cookie `_oauth_state` HttpOnly |
| H-01 / H-04 | ✅ | `SetCookieWithSameSite`; Secure só como flag |
| H-02 | ✅ | Whitelist sort; skip cols desconhecidas; escape `QuoteIdent` |
| H-03 | ✅ | `MinSecretLen=16`; `New` retorna erro / `MustNew` |
| M-01 | ✅ | Rotação de refresh + blacklist JTI em memória |
| M-02 | ✅ | `filepath.Rel` |
| M-03 | ✅ | `SafeRedirect` |
| M-04 | ✅ | `SetTrustedProxies` + `RemoteIP` |
| M-05 | ✅ | Warn stderr no `PublicStack` em produção |
| M-06 | ✅ parcial | `SaveFile` com `0600` |
| L-01 | ✅ | `InternalError` genérico |
| L-02 | ✅ | Request-ID sanitizado |
| L-04 | ✅ | Senha aleatória + `127.0.0.1` no compose |
| Deps Go | ⏳ | Toolchain do monorepo ainda mista; preferir Go ≥ 1.24.4 / 1.26 |

Testes de regressão: `adapters/fasthttp/security_fixes_test.go`, `adapters/auth/jwt/jwt_test.go`.

---

*Documento gerado a partir de revisão manual + govulncheck + gosec. Correções Critical/High aplicadas em 1.2.1. Não substitui pentest externo.*
