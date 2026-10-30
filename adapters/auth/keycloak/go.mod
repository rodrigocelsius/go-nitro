module github.com/rodrigocelsius/go-nitro/adapters/auth/keycloak/v2

go 1.22.2

require (
	github.com/rodrigocelsius/go-nitro/adapters/auth/oidc/v2 v2.2.0
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.2.0
	github.com/rodrigocelsius/go-nitro/ports/v2 v2.2.0
)

require (
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.2.0 // indirect
)

replace (
	github.com/rodrigocelsius/go-nitro/adapters/auth/oidc/v2 => ../oidc
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../../pkg/uuid
	github.com/rodrigocelsius/go-nitro/ports/v2 => ../../../ports
)
