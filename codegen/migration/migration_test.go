package migration

import (
	"os"
	"strings"
	"testing"

	nitroast "github.com/rodrigocelsius/go-nitro/codegen/ast/v2"
)

func TestGenerateMigration_BasicModel(t *testing.T) {
	migration, err := GenerateMigration("testdata/models.go")
	if err != nil {
		t.Fatalf("GenerateMigration failed: %v", err)
	}

	// Check UP contains CREATE TABLE
	if !strings.Contains(migration.Up, "CREATE TABLE") {
		t.Error("UP should contain CREATE TABLE")
	}

	// Check DOWN contains DROP TABLE
	if !strings.Contains(migration.Down, "DROP TABLE IF EXISTS") {
		t.Error("DOWN should contain DROP TABLE IF EXISTS")
	}

	// Check for users table
	if !strings.Contains(migration.Up, "users") {
		t.Error("UP should contain users table")
	}
}

func TestGenerateMigration_WithSchema(t *testing.T) {
	_, tables, err := GenerateMigrationWithSchema("testdata/models.go")
	if err != nil {
		t.Fatalf("GenerateMigrationWithSchema failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("Expected at least one table")
	}

	// Find users table
	var usersTable *TableSchema
	for i, table := range tables {
		if table.Name == "users" {
			usersTable = &tables[i]
			break
		}
	}

	if usersTable == nil {
		t.Fatal("Expected users table")
	}

	// Check columns
	if len(usersTable.Columns) == 0 {
		t.Fatal("Expected at least one column")
	}

	// Find email column
	var emailCol *ColumnSchema
	for i, col := range usersTable.Columns {
		if col.Name == "email" {
			emailCol = &usersTable.Columns[i]
			break
		}
	}

	if emailCol == nil {
		t.Fatal("Expected email column")
	}

	// Check unique constraint
	if !emailCol.Unique {
		t.Error("Email column should have UNIQUE constraint")
	}
}

func TestGenerateMigration_PrimaryKeyTag(t *testing.T) {
	_, tables, err := GenerateMigrationWithSchema("testdata/models.go")
	if err != nil {
		t.Fatalf("GenerateMigrationWithSchema failed: %v", err)
	}

	// Find products table
	var productsTable *TableSchema
	for i, table := range tables {
		if table.Name == "products" {
			productsTable = &tables[i]
			break
		}
	}

	if productsTable == nil {
		t.Fatal("Expected products table")
	}

	// Find id column
	var idCol *ColumnSchema
	for i, col := range productsTable.Columns {
		if col.Name == "id" {
			idCol = &productsTable.Columns[i]
			break
		}
	}

	if idCol == nil {
		t.Fatal("Expected id column")
	}

	// Check primary key
	if !idCol.PrimaryKey {
		t.Error("ID column should be PRIMARY KEY")
	}

	// Check type is BIGSERIAL (int PK auto-increment)
	if idCol.Type != TypeBigSerial {
		t.Errorf("Expected BIGSERIAL for int primary key, got %s", idCol.Type)
	}
}

func TestGenerateMigration_DefaultTag(t *testing.T) {
	_, tables, err := GenerateMigrationWithSchema("testdata/models.go")
	if err != nil {
		t.Fatalf("GenerateMigrationWithSchema failed: %v", err)
	}

	// Find users table
	var usersTable *TableSchema
	for i, table := range tables {
		if table.Name == "users" {
			usersTable = &tables[i]
			break
		}
	}

	if usersTable == nil {
		t.Fatal("Expected users table")
	}

	// Find role column
	var roleCol *ColumnSchema
	for i, col := range usersTable.Columns {
		if col.Name == "role" {
			roleCol = &usersTable.Columns[i]
			break
		}
	}

	if roleCol == nil {
		t.Fatal("Expected role column")
	}

	// Check default value
	if roleCol.Default != "'user'" {
		t.Errorf("Expected default value 'user', got %s", roleCol.Default)
	}
}

func TestGenerateMigration_TableOverride(t *testing.T) {
	_, tables, err := GenerateMigrationWithSchema("testdata/models.go")
	if err != nil {
		t.Fatalf("GenerateMigrationWithSchema failed: %v", err)
	}

	// Find custom_orders table (Order struct has //nitro:table=custom_orders)
	var customOrdersTable *TableSchema
	for _, table := range tables {
		if table.Name == "custom_orders" {
			customOrdersTable = &table
			break
		}
	}

	if customOrdersTable == nil {
		t.Fatal("Expected custom_orders table (from //nitro:table=override)")
	}
}

func TestGenerateMigration_Indexes(t *testing.T) {
	_, tables, err := GenerateMigrationWithSchema("testdata/models.go")
	if err != nil {
		t.Fatalf("GenerateMigrationWithSchema failed: %v", err)
	}

	// Find users table
	var usersTable *TableSchema
	for i, table := range tables {
		if table.Name == "users" {
			usersTable = &tables[i]
			break
		}
	}

	if usersTable == nil {
		t.Fatal("Expected users table")
	}

	// Check for unique index on email
	hasUniqueIndex := false
	for _, idx := range usersTable.Indexes {
		if idx.Unique && len(idx.Columns) == 1 && idx.Columns[0] == "email" {
			hasUniqueIndex = true
			break
		}
	}

	if !hasUniqueIndex {
		t.Error("Expected unique index on email column")
	}

	hasPartialPhone := false
	for _, idx := range usersTable.Indexes {
		if idx.Unique && len(idx.Columns) == 1 && idx.Columns[0] == "phone" &&
			idx.Where == "deleted_at IS NULL" {
			hasPartialPhone = true
		}
	}
	if !hasPartialPhone {
		t.Error("Expected partial unique index on phone WHERE deleted_at IS NULL")
	}

	hasActive := false
	hasCountry := false
	for _, idx := range usersTable.Indexes {
		if !idx.Unique && len(idx.Columns) == 1 && idx.Columns[0] == "active" {
			hasActive = true
		}
		if !idx.Unique && len(idx.Columns) == 1 && idx.Columns[0] == "country" {
			hasCountry = true
		}
	}
	if !hasActive || !hasCountry {
		t.Errorf("expected blinq:index on active/country; indexes=%+v", usersTable.Indexes)
	}
}

func TestGenerateMigrationFile(t *testing.T) {
	_, tables, err := GenerateMigrationWithSchema("testdata/models.go")
	if err != nil {
		t.Fatalf("GenerateMigrationWithSchema failed: %v", err)
	}

	content := GenerateMigrationFile("test_migration", tables)

	// Check header
	if !strings.Contains(content, "-- Migration: test_migration") {
		t.Error("Should contain migration header")
	}

	// Check for pgcrypto extension (inside Up)
	if !strings.Contains(content, `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`) {
		t.Error("Should contain pgcrypto extension")
	}
	upIdx := strings.Index(content, "-- +goose Up")
	extIdx := strings.Index(content, `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)
	if upIdx < 0 || extIdx < upIdx {
		t.Error("pgcrypto must be inside -- +goose Up section")
	}

	SetDialect(DialectMySQL)
	t.Cleanup(func() { SetDialect(DialectPostgres) })
	mysqlContent := GenerateMigrationFile("test_migration", tables)
	if strings.Contains(mysqlContent, "pgcrypto") {
		t.Error("MySQL migrations must not emit pgcrypto")
	}

	// Check for goose markers
	if !strings.Contains(content, "-- +goose Up") {
		t.Error("Should contain goose Up marker")
	}
	if !strings.Contains(content, "-- +goose Down") {
		t.Error("Should contain goose Down marker")
	}

	// Check that DOWN drops tables in reverse order
	downIdx := strings.Index(content, "-- +goose Down")
	if downIdx == -1 {
		t.Fatal("Should contain goose Down marker")
	}
	downSection := content[downIdx:]
	if !strings.Contains(downSection, "DROP TABLE IF EXISTS custom_orders;") {
		t.Error("DOWN should drop custom_orders (Order struct)")
	}
}

func TestGenerateMigrationFilename(t *testing.T) {
	name := GenerateMigrationFilename("create_hellos")
	if !strings.HasSuffix(name, "_create_hellos.sql") {
		t.Fatalf("suffix: %s", name)
	}
	prefix := strings.SplitN(name, "_", 2)[0]
	if len(prefix) != 14 {
		t.Fatalf("expected 14-digit timestamp prefix, got %q in %s", prefix, name)
	}
}

func TestMakeWritesFile(t *testing.T) {
	dir := t.TempDir()
	path, err := Make("testdata/models.go", MakeOptions{Dir: dir, Name: "create_users"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, dir) || !strings.HasSuffix(path, ".sql") {
		t.Fatalf("path=%s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, "-- +goose Up") || !strings.Contains(body, "CREATE TABLE IF NOT EXISTS") {
		t.Fatalf("bad content:\n%s", body)
	}
}

func TestFormatDefault(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		kind     nitroast.FieldKind
		expected string
	}{
		{"string value", "hello", nitroast.KindString, "'hello'"},
		{"bool true", "true", nitroast.KindBool, "TRUE"},
		{"bool false", "false", nitroast.KindBool, "FALSE"},
		{"int value", "42", nitroast.KindInt, "42"},
		{"float value", "3.14", nitroast.KindFloat, "3.14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDefault(tt.val, tt.kind)
			if result != tt.expected {
				t.Errorf("formatDefault(%q, %d) = %q, want %q", tt.val, tt.kind, result, tt.expected)
			}
		})
	}
}

func TestSQLNowDialects(t *testing.T) {
	t.Cleanup(func() { SetDialect(DialectPostgres) })

	SetDialect(DialectPostgres)
	if sqlNow() != "NOW()" {
		t.Fatalf("postgres: %s", sqlNow())
	}
	SetDialect(DialectMySQL)
	if sqlNow() != "CURRENT_TIMESTAMP(6)" {
		t.Fatalf("mysql: %s", sqlNow())
	}
	SetDialect(DialectSQLite)
	if sqlNow() != "CURRENT_TIMESTAMP" {
		t.Fatalf("sqlite: %s", sqlNow())
	}
	SetDialect(DialectSQLServer)
	if sqlNow() != "SYSUTCDATETIME()" {
		t.Fatalf("mssql: %s", sqlNow())
	}
}

func TestGoTypeToPostgres(t *testing.T) {
	tests := []struct {
		name     string
		kind     nitroast.FieldKind
		typeName string
		expected ColumnType
	}{
		{"string", nitroast.KindString, "string", TypeVarchar},
		{"int", nitroast.KindInt, "int", TypeBigInt},
		{"int32", nitroast.KindInt, "int32", TypeInt},
		{"int8", nitroast.KindInt, "int8", TypeSmallInt},
		{"float32", nitroast.KindFloat, "float32", TypeReal},
		{"float64", nitroast.KindFloat, "float64", TypeDouble},
		{"bool", nitroast.KindBool, "bool", TypeBoolean},
		{"time.Time", nitroast.KindTime, "Time", TypeTimestamp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := goTypeToPostgres(tt.kind, tt.typeName)
			if result != tt.expected {
				t.Errorf("goTypeToPostgres(%d, %q) = %q, want %q", tt.kind, tt.typeName, result, tt.expected)
			}
		})
	}
}
