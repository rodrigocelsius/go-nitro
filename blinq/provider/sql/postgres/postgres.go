// Package postgres is a thin helper: New(db) → sql.Provider with the Postgres dialect.
package postgres

import (
	blinqSQL "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
)

// New creates an SQL Provider with the PostgreSQL dialect.
func New(db blinqSQL.DB, opts ...blinqSQL.ProviderOption) *blinqSQL.Provider {
	return blinqSQL.New(db, dialect.Postgres(), opts...)
}
