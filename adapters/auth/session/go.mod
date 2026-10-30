module github.com/rodrigocelsius/go-nitro/adapters/auth/session/v2

go 1.26.0

require (
	github.com/redis/go-redis/v9 v9.23.0
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.2.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.2.0 // indirect
	go.uber.org/atomic v1.12.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)

replace (
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../../pkg/uuid
)
