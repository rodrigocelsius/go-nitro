module github.com/rodrigocelsius/go-nitro/adapters/log/v2

go 1.22.2

require github.com/rodrigocelsius/go-nitro/domain/v2 v2.0.1

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 v2.0.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.58.0 // indirect
)

replace github.com/rodrigocelsius/go-nitro/domain/v2 => ../../domain

replace github.com/rodrigocelsius/go-nitro/pkg/uuid/v2 => ../../pkg/uuid
