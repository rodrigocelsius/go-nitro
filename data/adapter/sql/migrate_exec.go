package sqladapter

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/migrate"
)

type migrateExec struct {
	db *sql.DB
}

func adaptMigrate(db *sql.DB) migrate.Executor {
	return &migrateExec{db: db}
}

func (e *migrateExec) Exec(ctx context.Context, query string, args []any) error {
	if e.db == nil {
		return fmt.Errorf("sqladapter: migrate db nil")
	}
	_, err := e.db.ExecContext(ctx, query, args...)
	return err
}

func (e *migrateExec) Query(ctx context.Context, query string, args []any) (migrate.Rows, error) {
	if e.db == nil {
		return nil, fmt.Errorf("sqladapter: migrate db nil")
	}
	rows, err := e.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return sqlMigrateRows{rows: rows}, nil
}

type sqlMigrateRows struct {
	rows *sql.Rows
}

func (r sqlMigrateRows) Next() bool             { return r.rows.Next() }
func (r sqlMigrateRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r sqlMigrateRows) Close()                 { _ = r.rows.Close() }
func (r sqlMigrateRows) Err() error             { return r.rows.Err() }
