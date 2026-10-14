# Auth Keycloak (Nitro v1)

Minimal example: Keycloak login/refresh + JWTMiddleware + RequireRole.

```bash
export KEYCLOAK_BASE_URL=http://localhost:8080
export KEYCLOAK_REALM=scheduler
export KEYCLOAK_CLIENT_ID=scheduler-api
export KEYCLOAK_CLIENT_SECRET=…

go run .   # API on :8081 (Keycloak stays on :8080)
```

- `POST /auth/login` `{ "username", "password" }`
- `POST /auth/refresh` `{ "refresh_token" }`
- `GET /api/me` (Bearer)
- `GET /api/admin/ping` (roles `admin` or `super_admin`)
