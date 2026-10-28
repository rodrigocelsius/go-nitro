module github.com/rodrigocelsius/go-nitro/adapters/auth/oauth2/v2

go 1.26.0

require (
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.1.0
	golang.org/x/oauth2 v0.37.0
)

require github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.1.0 // indirect

replace (
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../../pkg/uuid
)
