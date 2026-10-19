module github.com/rodrigocelsius/go-nitro/data/gateway/v2

go 1.22.2

replace (
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../pkg/uuid
)

require github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.9

require github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.9 // indirect
