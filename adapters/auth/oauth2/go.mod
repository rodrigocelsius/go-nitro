module github.com/rodrigocelsius/go-nitro/adapters/auth/oauth2/v2

go 1.25.0

require (
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.3
	golang.org/x/oauth2 v0.30.0
)

require github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.3 // indirect

replace (
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../../pkg/uuid
)
