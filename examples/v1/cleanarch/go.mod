module github.com/rodrigocelsius/go-nitro/examples/v1/cleanarch/v2

go 1.24.0

require github.com/rodrigocelsius/go-nitro/pkg/v2 v2.0.0

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.0 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/auth/bcrypt/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/auth/jwt/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/auth/keycloak/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/auth/oauth2/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/auth/oidc/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/auth/session/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/cache/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/adapters/otel/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/codegen/openapi/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/ports/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/scheduler/v2 v2.0.0 // indirect
	github.com/rodrigocelsius/go-nitro/usecases/v2 v2.0.0 // indirect
	github.com/redis/go-redis/v9 v9.7.3 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.58.0 // indirect
	go.opentelemetry.io/otel v1.24.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.24.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.24.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.24.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.24.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.24.0 // indirect
	go.opentelemetry.io/otel/metric v1.24.0 // indirect
	go.opentelemetry.io/otel/sdk v1.24.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.24.0 // indirect
	go.opentelemetry.io/otel/trace v1.24.0 // indirect
	go.opentelemetry.io/proto/otlp v1.1.0 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/oauth2 v0.25.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240102182953-50ed04b92917 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240102182953-50ed04b92917 // indirect
	google.golang.org/grpc v1.61.1 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
)

replace (
	github.com/rodrigocelsius/go-nitro/adapters/auth/bcrypt/v2 => ../../../adapters/auth/bcrypt
	github.com/rodrigocelsius/go-nitro/adapters/auth/jwt/v2 => ../../../adapters/auth/jwt
	github.com/rodrigocelsius/go-nitro/adapters/auth/keycloak/v2 => ../../../adapters/auth/keycloak
	github.com/rodrigocelsius/go-nitro/adapters/auth/oauth2/v2 => ../../../adapters/auth/oauth2
	github.com/rodrigocelsius/go-nitro/adapters/auth/oidc/v2 => ../../../adapters/auth/oidc
	github.com/rodrigocelsius/go-nitro/adapters/auth/session/v2 => ../../../adapters/auth/session
	github.com/rodrigocelsius/go-nitro/adapters/cache/v2 => ../../../adapters/cache
	github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2 => ../../../adapters/fasthttp
	github.com/rodrigocelsius/go-nitro/adapters/otel/v2 => ../../../adapters/otel
	github.com/rodrigocelsius/go-nitro/adapters/ratelimit/v2 => ../../../adapters/ratelimit
	github.com/rodrigocelsius/go-nitro/codegen/openapi/v2 => ../../../codegen/openapi
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../../domain
	github.com/rodrigocelsius/go-nitro/pkg/v2 => ../../../pkg
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../../pkg/uuid
	github.com/rodrigocelsius/go-nitro/ports/v2 => ../../../ports
	github.com/rodrigocelsius/go-nitro/scheduler/v2 => ../../../scheduler
	github.com/rodrigocelsius/go-nitro/usecases/v2 => ../../../usecases
	github.com/rodrigocelsius/go-nitro/adapters/amqp/v2 => ../../../adapters/amqp
	github.com/rodrigocelsius/go-nitro/queue/v2 => ../../../queue
	github.com/rodrigocelsius/go-nitro/resilience/v2 => ../../../resilience
)
