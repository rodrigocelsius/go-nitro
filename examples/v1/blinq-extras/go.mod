module github.com/rodrigocelsius/go-nitro/examples/v1/blinq-extras/v2

go 1.25.0

require (
	github.com/rodrigocelsius/go-nitro/blinq/v2 v2.0.0
	github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2 v2.0.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/rodrigocelsius/go-nitro/data/gateway/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace (
	github.com/rodrigocelsius/go-nitro/blinq/v2 => ../../../blinq
	github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2 => ../../../data/adapter/pgx
	github.com/rodrigocelsius/go-nitro/data/gateway/v2 => ../../../data/gateway
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../../pkg/uuid
	github.com/rodrigocelsius/go-nitro/adapters/amqp/v2 => ../../../adapters/amqp
	github.com/rodrigocelsius/go-nitro/adapters/cache/v2 => ../../../adapters/cache
	github.com/rodrigocelsius/go-nitro/adapters/otel/v2 => ../../../adapters/otel
	github.com/rodrigocelsius/go-nitro/adapters/auth/keycloak/v2 => ../../../adapters/auth/keycloak
	github.com/rodrigocelsius/go-nitro/adapters/auth/oidc/v2 => ../../../adapters/auth/oidc
	github.com/rodrigocelsius/go-nitro/queue/v2 => ../../../queue
	github.com/rodrigocelsius/go-nitro/resilience/v2 => ../../../resilience
)
