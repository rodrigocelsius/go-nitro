// CRUD example — Nitro v1 syntax.
//
// Demonstrates: MemoryRepo, app.Resource (path by convention), nitro.Model (UUID v7 + soft delete),
// default cache (5m). Update via PATCH.
//
// Run:
//
//	go run main.go
//
// Test:
//
//	curl http://localhost:8080/todos
//	curl -X POST http://localhost:8080/todos -d '{"title":"Buy milk"}'
//	curl http://localhost:8080/todos/<id>
//	curl -X PATCH http://localhost:8080/todos/<id> -d '{"title":"Buy coffee","done":true}'
//	curl -X DELETE http://localhost:8080/todos/<id>
package main

import (
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// Todo — id/timestamps/soft delete via nitro.Model.
type Todo struct {
	nitro.Model
	Title string `json:"title" validate:"required,min=2"`
	Done  bool   `json:"done"`
}

func main() {
	app := nitro.New()
	// Convention: the path is the plural of the model → /todos.
	app.Resource(&Todo{}, nitro.NewMemoryRepo[Todo]())

	nitro.Ok(app.Listen(":8080"))
}
