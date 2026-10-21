module github.com/rodrigocelsius/go-nitro/adapters/auth/bcrypt/v2

go 1.26.0

require (
	github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.10
	golang.org/x/crypto v0.58.0
)

require github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.10 // indirect

replace (
	github.com/rodrigocelsius/go-nitro/domain/v2 => ../../../domain
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../../pkg/uuid
)
