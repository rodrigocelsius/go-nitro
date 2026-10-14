module github.com/rodrigocelsius/go-nitro/adapters/auth/bcrypt/v2

go 1.22.2

require (
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.0
	golang.org/x/crypto v0.31.0
)

require github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.0 // indirect

replace (
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../../pkg/uuid
)
