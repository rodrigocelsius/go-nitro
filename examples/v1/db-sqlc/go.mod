module github.com/rodrigocelsius/go-nitro/examples/v1/db-sqlc/v2

go 1.25.0

require (
	github.com/jackc/pgx/v5 v5.10.0
	github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2 v2.0.0
	github.com/rodrigocelsius/go-nitro/data/gateway/v2 v2.0.0
	github.com/rodrigocelsius/go-nitro/pkg/v2 v2.0.0
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/auth/bcrypt/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/auth/jwt/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/auth/oauth2/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/auth/session/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/blinq/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/codegen/openapi/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/ports/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/scheduler/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/usecases/v2 v2.0.0 // indirect
	github.com/redis/go-redis/v9 v9.7.3 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.58.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/oauth2 v0.25.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace (
	github.com/rodrigocelsius/go-nitro/adapters/auth/bcrypt/v2 => ../../../adapters/auth/bcrypt
	github.com/rodrigocelsius/go-nitro/adapters/auth/jwt/v2 => ../../../adapters/auth/jwt
	github.com/rodrigocelsius/go-nitro/adapters/auth/oauth2/v2 => ../../../adapters/auth/oauth2
	github.com/rodrigocelsius/go-nitro/adapters/auth/session/v2 => ../../../adapters/auth/session
	github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2 => ../../../adapters/fasthttp
	github.com/rodrigocelsius/go-nitro/adapters/ratelimit/v2 => ../../../adapters/ratelimit
	github.com/rodrigocelsius/go-nitro/blinq/v2 => ../../../blinq
	github.com/rodrigocelsius/go-nitro/codegen/openapi/v2 => ../../../codegen/openapi
	github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2 => ../../../data/adapter/pgx
	github.com/rodrigocelsius/go-nitro/data/gateway/v2 => ../../../data/gateway
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../../domain
	github.com/rodrigocelsius/go-nitro/pkg/v2 => ../../../pkg
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../../pkg/uuid
	github.com/rodrigocelsius/go-nitro/ports/v2 => ../../../ports
	github.com/rodrigocelsius/go-nitro/scheduler/v2 => ../../../scheduler
	github.com/rodrigocelsius/go-nitro/usecases/v2 => ../../../usecases
	github.com/rodrigocelsius/go-nitro/adapters/amqp/v2 => ../../../adapters/amqp
	github.com/rodrigocelsius/go-nitro/adapters/cache/v2 => ../../../adapters/cache
	github.com/rodrigocelsius/go-nitro/adapters/otel/v2 => ../../../adapters/otel
	github.com/rodrigocelsius/go-nitro/adapters/auth/keycloak/v2 => ../../../adapters/auth/keycloak
	github.com/rodrigocelsius/go-nitro/adapters/auth/oidc/v2 => ../../../adapters/auth/oidc
	github.com/rodrigocelsius/go-nitro/queue/v2 => ../../../queue
	github.com/rodrigocelsius/go-nitro/resilience/v2 => ../../../resilience
)
