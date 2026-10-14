package sqladapter

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
)

func TestDialectOf(t *testing.T) {
	if dialectOf(DriverMySQL).Name() != dialect.MySQL().Name() {
		t.Fatal(dialectOf(DriverMySQL).Name())
	}
	if dialectOf(DriverMariaDB).ReturningSupported() {
		t.Fatal("mariadb must not use RETURNING")
	}
	if dialectOf(DriverSQLServer).Name() != dialect.MSSQL().Name() {
		t.Fatal(dialectOf(DriverSQLServer).Name())
	}
	if dialectOf(DriverSQLite).Name() != dialect.SQLite().Name() {
		t.Fatal(dialectOf(DriverSQLite).Name())
	}
}
