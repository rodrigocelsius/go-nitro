module github.com/rodrigocelsius/go-nitro/codegen/migration/v2

go 1.22.2

require (
	github.com/rodrigocelsius/go-nitro/codegen/ast/v2 v2.0.1
	github.com/rodrigocelsius/go-nitro/codegen/data/v2 v2.0.1
)

replace (
	github.com/rodrigocelsius/go-nitro/codegen/ast/v2 => ../ast
	github.com/rodrigocelsius/go-nitro/codegen/data/v2 => ../data
)