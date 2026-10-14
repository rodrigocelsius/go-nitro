package pgx

import (
	"fmt"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

// Attach associates the DB with the App (once). Resource[T] binds lazily — no explicit Bind per model.
//
//	pgx.Attach(app, db)
//	pgx.Resource[Driver](api, "/drivers", &nitro.ResourceConfig{Filterable: []string{"name"}})
func Attach(app *nitro.App, db *DB) {
	if app == nil || db == nil {
		return
	}
	app.SetSQL(db)
}

// From returns the DB from Attach (or nil).
func From(app *nitro.App) *DB {
	if app == nil {
		return nil
	}
	db, _ := app.SQL().(*DB)
	return db
}

// Resource builds CRUD with an implicit Store (table = last path segment).
// Requires Attach(app, db) first.
func Resource[T any](api *nitro.Group, path string, cfg *nitro.ResourceConfig) {
	if api == nil {
		panic("pgx.Resource: group is nil")
	}
	app := api.App()
	db := From(app)
	if db == nil {
		panic("pgx.Resource: chame pgx.Attach(app, db) no bootstrap")
	}
	Bind[T](app, db)
	var model T
	if cfg == nil {
		cfg = &nitro.ResourceConfig{}
	}
	api.Resource(path, &model, cfg)
}

// Bind registers model T on the app (optional if using Resource[T]).
func Bind[T any](app *nitro.App, db *DB) {
	nitro.Bind[T](app, func(table string) domain.ResourceRepository {
		return AdaptStore(StoreOf[T](db, table))
	})
}

// Repo is a shortcut for AdaptStore(StoreOf[T](db, table)).
func Repo[T any](db *DB, table string) domain.ResourceRepository {
	return AdaptStore(StoreOf[T](db, table))
}

// MustDB panics if Attach was not called.
func MustDB(app *nitro.App) *DB {
	db := From(app)
	if db == nil {
		panic(fmt.Errorf("pgx: Attach(app, db) required"))
	}
	return db
}
