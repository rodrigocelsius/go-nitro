module github.com/rodrigocelsius/go-nitro/usecases/v2

go 1.22.2

require (
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.1.0
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.1.0
	github.com/rodrigocelsius/go-nitro/ports/v2 v2.1.0
)

replace (
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../pkg/uuid
	github.com/rodrigocelsius/go-nitro/ports/v2 => ../ports
)
