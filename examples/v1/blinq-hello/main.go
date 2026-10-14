// BLinq hello world — OpenEnv + migrate + Store + Resource (CRUD without handlers).
//
//	nitrogen migrate:make models.go --name create_hellos
//	export DATABASE_URL='postgres://postgres:postgres@localhost:5432/nitro?sslmode=disable'
//	go run .
package main

import (
	"context"
	"log"

	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func main() {
	ctx := context.Background()
	db, err := pgxadapter.OpenEnv(ctx) // DATABASE_URL | POSTGRES_DSN
	if err != nil {
		log.Fatalf("db: %v (set DATABASE_URL)", err)
	}
	defer db.Close()

	if err := db.MigrateUp(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	store := pgxadapter.StoreOf[Hello](db, "hellos")

	app := nitro.New()
	// Path from TableName() / //nitro:table=hellos → /hellos; store used directly (no AdaptStore).
	app.Resource(&Hello{}, store, &nitro.ResourceConfig{NoCache: true})
	nitro.Ok(app.Listen(":8080"))
}
