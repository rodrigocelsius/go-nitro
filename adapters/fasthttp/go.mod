module github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2

go 1.26.0

require (
	github.com/rodrigocelsius/go-nitro/adapters/ratelimit/v2 v2.1.0
	github.com/rodrigocelsius/go-nitro/codegen/openapi/v2 v2.1.0
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.1.0
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.1.0
	github.com/rodrigocelsius/go-nitro/ports/v2 v2.1.0
	github.com/rodrigocelsius/go-nitro/usecases/v2 v2.1.0
	github.com/valyala/fasthttp v1.76.0
	golang.org/x/crypto v0.58.0
)

require (
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/molecule-man/go-brrr v1.3.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

replace (
	github.com/rodrigocelsius/go-nitro/adapters/ratelimit/v2 => ../ratelimit
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../domain
	github.com/rodrigocelsius/go-nitro/ports/v2 => ../../ports
	github.com/rodrigocelsius/go-nitro/usecases/v2 => ../../usecases
)

replace github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../pkg/uuid

replace github.com/rodrigocelsius/go-nitro/codegen/openapi/v2 => ../../codegen/openapi
