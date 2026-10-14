module github.com/rodrigocelsius/go-nitro/cmd/nitrogen/v2

go 1.22.2

replace github.com/rodrigocelsius/go-nitro/codegen/json/v2 => ../../codegen/json

replace github.com/rodrigocelsius/go-nitro/codegen/ast/v2 => ../../codegen/ast

replace github.com/rodrigocelsius/go-nitro/codegen/data/v2 => ../../codegen/data

replace github.com/rodrigocelsius/go-nitro/codegen/blinq/v2 => ../../codegen/blinq

replace github.com/rodrigocelsius/go-nitro/codegen/migration/v2 => ../../codegen/migration

replace github.com/rodrigocelsius/go-nitro/codegen/controller/v2 => ../../codegen/controller

replace github.com/rodrigocelsius/go-nitro/codegen/dto/v2 => ../../codegen/dto

replace github.com/rodrigocelsius/go-nitro/codegen/openapi/v2 => ../../codegen/openapi

require github.com/rodrigocelsius/go-nitro/codegen/json/v2 v2.0.0

require github.com/rodrigocelsius/go-nitro/codegen/data/v2 v2.0.0

require github.com/rodrigocelsius/go-nitro/codegen/blinq/v2 v2.0.0

require github.com/rodrigocelsius/go-nitro/codegen/migration/v2 v2.0.0

require github.com/rodrigocelsius/go-nitro/codegen/controller/v2 v2.0.0

require github.com/rodrigocelsius/go-nitro/codegen/dto/v2 v2.0.0

require github.com/rodrigocelsius/go-nitro/codegen/openapi/v2 v2.0.0

require github.com/rodrigocelsius/go-nitro/codegen/ast/v2 v2.0.0 // indirect
