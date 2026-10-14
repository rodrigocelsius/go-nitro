//go:build integration

package sqladapter_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type productRow struct {
	nitro.Model
	Name   string  `db:"name"`
	Price  float64 `db:"price"`
	Active bool    `db:"active"`
}

func TestLiveDrivers_CRUD(t *testing.T) {
	if os.Getenv("NITRO_SQL_INTEGRATION") != "1" {
		t.Skip("set NITRO_SQL_INTEGRATION=1 (see make test-sql-integration)")
	}

	cases := []struct {
		name   string
		driver string
		dsn    string
		schema string
	}{
		{
			name:   "sqlite",
			driver: "sqlite",
			dsn:    "file:" + filepath.Join(t.TempDir(), "live.db"),
			schema: `CREATE TABLE IF NOT EXISTS products (
  id TEXT PRIMARY KEY NOT NULL,
  name TEXT NOT NULL,
  price REAL NOT NULL,
  active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);`,
		},
		{
			name:   "mysql",
			driver: "mysql",
			dsn:    envOr("NITRO_MYSQL_DSN", "user:nitro@tcp(127.0.0.1:3307)/nitro?parseTime=true&loc=UTC"),
			schema: `CREATE TABLE IF NOT EXISTS products (
  id CHAR(36) PRIMARY KEY NOT NULL,
  name VARCHAR(255) NOT NULL,
  price DOUBLE NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  deleted_at DATETIME(6) NULL
);`,
		},
		{
			name:   "mariadb",
			driver: "mariadb",
			dsn:    envOr("NITRO_MARIADB_DSN", "user:nitro@tcp(127.0.0.1:3308)/nitro?parseTime=true&loc=UTC"),
			schema: `CREATE TABLE IF NOT EXISTS products (
  id CHAR(36) PRIMARY KEY NOT NULL,
  name VARCHAR(255) NOT NULL,
  price DOUBLE NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  deleted_at DATETIME(6) NULL
);`,
		},
		{
			name:   "sqlserver",
			driver: "sqlserver",
			dsn:    envOr("NITRO_MSSQL_DSN", "sqlserver://sa:Nitro_Str0ng!@127.0.0.1:1434?database=nitro"),
			schema: `IF OBJECT_ID(N'products', N'U') IS NULL CREATE TABLE products (
  id NVARCHAR(36) PRIMARY KEY NOT NULL,
  name NVARCHAR(255) NOT NULL,
  price FLOAT NOT NULL,
  active BIT NOT NULL DEFAULT 1,
  created_at DATETIMEOFFSET NOT NULL,
  updated_at DATETIMEOFFSET NOT NULL,
  deleted_at DATETIMEOFFSET NULL
);`,
		},
		{
			name:   "postgresql",
			driver: "postgresql",
			dsn:    envOr("NITRO_PG_DSN", "postgres://postgres:nitro@127.0.0.1:15432/nitro?sslmode=disable"),
			schema: `CREATE TABLE IF NOT EXISTS products (
  id TEXT PRIMARY KEY NOT NULL,
  name VARCHAR(255) NOT NULL,
  price DOUBLE PRECISION NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  deleted_at TIMESTAMPTZ
);`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			_ = os.Setenv("NITRO_DB_QUIET", "1")
			db, err := sqladapter.Open(ctx, tc.driver, tc.dsn)
			if err != nil {
				if os.Getenv("NITRO_SQL_REQUIRE") == "1" { // CI: a database that is down must fail, not skip
					t.Fatalf("open %s: %v", tc.name, err)
				}
				t.Skipf("open %s: %v (is the docker service up?)", tc.name, err)
			}
			defer db.Close()

			if err := execSchema(ctx, db, tc.schema); err != nil {
				t.Fatalf("schema: %v", err)
			}
			if _, err := db.Gateway().Exec(ctx, "delete", "DELETE FROM products", nil); err != nil {
				t.Fatalf("cleanup: %v", err)
			}

			store := sqladapter.StoreOf[productRow](db, "products")
			created, err := store.CreateOne(ctx, &productRow{
				Name:   "Widget-" + tc.name,
				Price:  9.5,
				Active: true,
			})
			if err != nil {
				t.Fatalf("CreateOne: %v", err)
			}
			if created.ID.IsZero() || created.Name == "" {
				t.Fatalf("bad create: %#v", created)
			}
			if created.CreatedAt.IsZero() {
				t.Fatalf("expected CreatedAt: %#v", created)
			}

			got, err := store.Get(ctx, created.ID)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.Name != created.Name {
				t.Fatalf("get name=%q", got.Name)
			}

			sets := []expr.Assignment{
				field.Col[string]("products", "name").Set("Gadget-" + tc.name),
				field.Col[float64]("products", "price").Set(1.25),
				field.Col[bool]("products", "active").Set(false),
			}
			updated, err := store.Update(ctx, got.ID, sets...)
			if err != nil {
				t.Fatalf("Update: %v", err)
			}
			if updated.Name != "Gadget-"+tc.name || updated.Active {
				t.Fatalf("update: %#v", updated)
			}

			items, err := store.Find(ctx, store.Query().Limit(10))
			if err != nil {
				t.Fatalf("Find: %v", err)
			}
			if len(items) < 1 {
				t.Fatal("expected rows")
			}

			if err := store.Delete(ctx, created.ID); err != nil {
				t.Fatalf("Delete: %v", err)
			}
			if _, err := store.Get(ctx, created.ID); err == nil {
				t.Fatal("expected not found after delete")
			}
		})
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func execSchema(ctx context.Context, db *sqladapter.DB, schema string) error {
	if db.Pgx() != nil {
		return db.Pgx().ExecSQL(ctx, schema)
	}
	std := db.StdDB()
	if std == nil {
		return fmt.Errorf("no executor")
	}
	_, err := std.ExecContext(ctx, schema)
	return err
}
