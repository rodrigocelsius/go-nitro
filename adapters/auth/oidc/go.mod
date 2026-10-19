module github.com/rodrigocelsius/go-nitro/adapters/auth/oidc/v2

go 1.22.2

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.5
	github.com/rodrigocelsius/go-nitro/ports/v2 v2.0.5
)

require github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.5 // indirect

replace (
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../../pkg/uuid
	github.com/rodrigocelsius/go-nitro/ports/v2 => ../../../ports
)
