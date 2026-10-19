module github.com/rodrigocelsius/go-nitro/codegen/json/v2

go 1.22.2

replace (
	github.com/rodrigocelsius/go-nitro/codegen/ast/v2 => ../ast
	github.com/rodrigocelsius/go-nitro/json/runtime/v2 => ./runtime
)

require github.com/rodrigocelsius/go-nitro/codegen/ast/v2 v2.0.6
