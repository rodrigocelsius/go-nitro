package migration

// Dialect identifies the target SQL dialect for column type mapping.
type Dialect string

const (
	DialectPostgres  Dialect = "postgres"
	DialectMySQL     Dialect = "mysql"
	DialectMariaDB   Dialect = "mariadb"
	DialectSQLite    Dialect = "sqlite"
	DialectSQLServer Dialect = "sqlserver"
)

// DefaultDialect is used when none is configured.
var DefaultDialect = DialectPostgres

// currentDialect is set by nitrogen install / NITRO_DB_DRIVER.
var currentDialect = DefaultDialect

// SetDialect sets the active SQL dialect for codegen.
func SetDialect(d Dialect) {
	switch Dialect(stringsToLower(string(d))) {
	case DialectMySQL, "mariadb":
		currentDialect = DialectMySQL
	case DialectSQLite, "sqlite3":
		currentDialect = DialectSQLite
	case DialectSQLServer, "mssql":
		currentDialect = DialectSQLServer
	default:
		currentDialect = DialectPostgres
	}
}

// CurrentDialect returns the active dialect.
func CurrentDialect() Dialect { return currentDialect }

// DialectFromDriver maps driver/env names to Dialect.
func DialectFromDriver(driver string) Dialect {
	switch stringsToLower(driver) {
	case "mysql", "mariadb":
		return DialectMySQL
	case "sqlite", "sqlite3":
		return DialectSQLite
	case "sqlserver", "mssql":
		return DialectSQLServer
	default:
		return DialectPostgres
	}
}

func stringsToLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// Extra column type constants for multi-dialect.
const (
	TypeChar36         ColumnType = "CHAR(36)"
	TypeUniqueIdent    ColumnType = "UNIQUEIDENTIFIER"
	TypeMySQLJSON      ColumnType = "JSON"
	TypeMySQLDateTime  ColumnType = "DATETIME(6)"
	TypeDateTimeOffset ColumnType = "DATETIMEOFFSET"
	TypeNVarcharMax    ColumnType = "NVARCHAR(MAX)"
	TypeDoublePlain    ColumnType = "DOUBLE"
	TypeNumeric3810    ColumnType = "NUMERIC(38,10)"
	TypeDecimal3810    ColumnType = "DECIMAL(38,10)"
	TypeChar3          ColumnType = "CHAR(3)"
	TypeVarchar16      ColumnType = "VARCHAR(16)"
	TypeNumeric129     ColumnType = "NUMERIC(12,9)"
	TypeDecimal129     ColumnType = "DECIMAL(12,9)"
)
