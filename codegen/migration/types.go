package migration

// ColumnType represents a PostgreSQL column type
type ColumnType string

const (
	TypeUUID      ColumnType = "UUID"
	TypeVarchar   ColumnType = "VARCHAR(255)"
	TypeText      ColumnType = "TEXT"
	TypeBigInt    ColumnType = "BIGINT"
	TypeBigSerial ColumnType = "BIGSERIAL"
	TypeInt       ColumnType = "INTEGER"
	TypeSmallInt  ColumnType = "SMALLINT"
	TypeReal      ColumnType = "REAL"
	TypeDouble    ColumnType = "DOUBLE PRECISION"
	TypeBoolean   ColumnType = "BOOLEAN"
	TypeTimestamp ColumnType = "TIMESTAMPTZ"
	TypeDate      ColumnType = "DATE"
	TypeTime      ColumnType = "TIME"
	TypeJSON      ColumnType = "JSONB"
	TypeJSONB     ColumnType = "JSONB"
	TypeDecimal   ColumnType = "DECIMAL"
	TypeNumeric   ColumnType = "NUMERIC"
)

// ColumnSchema represents a column in a database table
type ColumnSchema struct {
	Name       string
	Type       ColumnType
	NotNull    bool
	Unique     bool
	PrimaryKey bool
	Default    string
	Nullable   bool
	Comment    string
}

// IndexSchema represents an index on a table
type IndexSchema struct {
	Name    string
	Columns []string
	Unique  bool
	Where   string // partial index condition
	Using   string // e.g. GIN for JSONB
}

// TableSchema represents a database table
type TableSchema struct {
	Name       string
	Comment    string
	Columns    []ColumnSchema
	Indexes    []IndexSchema
	PrimaryKey []string
}

// Migration represents a SQL migration
type Migration struct {
	Up   string
	Down string
}
