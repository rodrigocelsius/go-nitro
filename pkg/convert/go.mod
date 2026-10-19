module github.com/rodrigocelsius/go-nitro/pkg/convert/v2

go 1.22.2

require (
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.5
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.5
)

replace (
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../uuid
)
