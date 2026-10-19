module github.com/rodrigocelsius/go-nitro/ports/v2

go 1.22.2

require github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.9

require github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.9 // indirect

replace github.com/rodrigocelsius/go-nitro/domain/v2 => ../domain

replace github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../pkg/uuid
