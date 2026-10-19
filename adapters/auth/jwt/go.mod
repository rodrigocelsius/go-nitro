module github.com/rodrigocelsius/go-nitro/adapters/auth/jwt/v2

go 1.22.2

replace (
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../../pkg/uuid
)

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.4
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.4
)
