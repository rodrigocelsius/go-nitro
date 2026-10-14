# Security Policy

## Secure by default

`nitro.New()` enables:

- Security headers (CSP, nosniff, frame deny, HSTS in production)
- Request ID
- Panic recovery
- Access logger
- 1 MiB max body
- Development CORS (`*`) when `EnvDevelopment`
- Resource in-memory cache (5m) when using `Resource`
- Graceful shutdown on `Listen`

**Opt-in for APIs under abuse:** `app.UseStack(nitro.APIStack)` (RateLimit + Compress).

**Browser cookie forms:** CSRF via Web BFF or `StandaloneCSRFMiddleware` (not default for JSON APIs).

**Production CORS:** set `Config.Security.CORS.AllowedOrigins` (avoid `PublicStack` `*` in production when possible).

Technical audit (method, findings, mitigations): [`docs/security/AUDIT.md`](docs/security/AUDIT.md).

## Reporting a Vulnerability


If you discover a security vulnerability in Nitro, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email: `security@nitro.dev` (or use GitHub's private vulnerability reporting feature).

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 1 week
- **Fix or mitigation**: Within 30 days for critical vulnerabilities

## Scope

The following are in scope:
- The `github.com/rodrigocelsius/go-nitro/v2` Go module
- Built-in middleware (CORS, CSRF, JWT, Session, Rate Limit)
- Authentication adapters (JWT, OAuth2, Session, bcrypt)
- The fasthttp adapter

The following are out of scope:
- Third-party dependencies (report to their maintainers)
- Applications built with Nitro (report to the application developers)
- Social engineering attacks

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |
| < Latest | No       |

We recommend always running the latest version.
