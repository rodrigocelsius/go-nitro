package sqladapter

import (
	"strings"
)

// Driver is a SQL engine name used by OpenEnv / NITRO_DB_DRIVER.
type Driver string

const (
	DriverPostgres  Driver = "postgresql"
	DriverMySQL     Driver = "mysql"
	DriverMariaDB   Driver = "mariadb"
	DriverSQLServer Driver = "sqlserver"
	DriverSQLite    Driver = "sqlite"
)

func normalizeDriver(s string) Driver {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "postgres", "postgresql", "pg", "pgsql":
		return DriverPostgres
	case "mysql":
		return DriverMySQL
	case "maria", "mariadb":
		return DriverMariaDB
	case "sqlserver", "mssql", "sql-server":
		return DriverSQLServer
	case "sqlite", "sqlite3":
		return DriverSQLite
	default:
		return ""
	}
}

func detectDriver(explicit, dsn string) Driver {
	if d := normalizeDriver(explicit); d != "" {
		return d
	}
	u := strings.ToLower(strings.TrimSpace(dsn))
	switch {
	case strings.HasPrefix(u, "postgres://"), strings.HasPrefix(u, "postgresql://"):
		return DriverPostgres
	case strings.HasPrefix(u, "sqlserver://"), strings.HasPrefix(u, "server="):
		return DriverSQLServer
	case strings.HasPrefix(u, "file:"), strings.HasSuffix(u, ".db"), strings.Contains(u, "mode=memory"):
		return DriverSQLite
	case strings.Contains(u, "@tcp("), strings.HasPrefix(u, "mysql://"):
		return DriverMySQL
	default:
		return DriverPostgres
	}
}

func stdDriverName(d Driver) string {
	switch d {
	case DriverMySQL, DriverMariaDB:
		return "mysql"
	case DriverSQLServer:
		return "sqlserver"
	case DriverSQLite:
		return "sqlite"
	default:
		return ""
	}
}
