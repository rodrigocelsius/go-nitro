module github.com/rodrigocelsius/go-nitro/v2

go 1.22.2

// === Clean Architecture Packages ===
// Domain (core types, zero dependencies)
replace github.com/rodrigocelsius/go-nitro/domain/v2 => ./domain

// Ports (interfaces)
replace github.com/rodrigocelsius/go-nitro/ports/v2 => ./ports

// Usecases (framework logic)
replace github.com/rodrigocelsius/go-nitro/usecases/v2 => ./usecases

// Adapters (concrete implementations)
replace github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2 => ./adapters/fasthttp

replace github.com/rodrigocelsius/go-nitro/adapters/ratelimit/v2 => ./adapters/ratelimit

replace github.com/rodrigocelsius/go-nitro/adapters/cache/v2 => ./adapters/cache

replace github.com/rodrigocelsius/go-nitro/adapters/log/v2 => ./adapters/log

replace github.com/rodrigocelsius/go-nitro/adapters/otel/v2 => ./adapters/otel

replace github.com/rodrigocelsius/go-nitro/adapters/amqp/v2 => ./adapters/amqp

replace github.com/rodrigocelsius/go-nitro/adapters/auth/bcrypt/v2 => ./adapters/auth/bcrypt

replace github.com/rodrigocelsius/go-nitro/adapters/auth/jwt/v2 => ./adapters/auth/jwt

replace github.com/rodrigocelsius/go-nitro/adapters/auth/session/v2 => ./adapters/auth/session

replace github.com/rodrigocelsius/go-nitro/adapters/auth/oauth2/v2 => ./adapters/auth/oauth2

replace github.com/rodrigocelsius/go-nitro/adapters/auth/oidc/v2 => ./adapters/auth/oidc

replace github.com/rodrigocelsius/go-nitro/adapters/auth/keycloak/v2 => ./adapters/auth/keycloak

// Codegen (code generation tools)
replace github.com/rodrigocelsius/go-nitro/codegen/ast/v2 => ./codegen/ast

replace github.com/rodrigocelsius/go-nitro/codegen/json/v2 => ./codegen/json

replace github.com/rodrigocelsius/go-nitro/codegen/data/v2 => ./codegen/data

replace github.com/rodrigocelsius/go-nitro/codegen/blinq/v2 => ./codegen/blinq

replace github.com/rodrigocelsius/go-nitro/codegen/router/v2 => ./codegen/router

replace github.com/rodrigocelsius/go-nitro/codegen/openapi/v2 => ./codegen/openapi

// Facade (public API)
replace github.com/rodrigocelsius/go-nitro/pkg/v2 => ./pkg

replace github.com/rodrigocelsius/go-nitro/adapters/kafka/v2 => ./adapters/kafka
replace github.com/rodrigocelsius/go-nitro/adapters/nats/v2 => ./adapters/nats
replace github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ./pkg/uuid

// Data layer
replace github.com/rodrigocelsius/go-nitro/blinq/v2 => ./blinq

replace github.com/rodrigocelsius/go-nitro/data/gateway/v2 => ./data/gateway

replace github.com/rodrigocelsius/go-nitro/data/repository/v2 => ./data/repository

replace github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2 => ./data/adapter/pgx

replace github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2 => ./data/adapter/sql

// Other packages

replace github.com/rodrigocelsius/go-nitro/queue/v2 => ./queue

replace github.com/rodrigocelsius/go-nitro/scheduler/v2 => ./scheduler

replace github.com/rodrigocelsius/go-nitro/resilience/v2 => ./resilience
