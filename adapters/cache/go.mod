module github.com/rodrigocelsius/go-nitro/adapters/cache/v2

go 1.25.0

require (
	github.com/alicebob/miniredis/v2 v2.34.0
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.4
	github.com/redis/go-redis/v9 v9.22.0
)

require (
	github.com/alicebob/gopher-json v0.0.0-20230218143504-906a9b012302 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.4 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)

replace (
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../pkg/uuid
)
