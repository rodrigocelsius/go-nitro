package migration

import "testing"

func TestGoTypeToSQL_Portable(t *testing.T) {
	SetDialect(DialectPostgres)
	if got := goTypeToSQL(0, "nitro.Date"); got != TypeDate {
		t.Fatalf("Date: %s", got)
	}
	if got := goTypeToSQL(0, "UUID"); got != TypeUUID {
		t.Fatalf("UUID: %s", got)
	}
	SetDialect(DialectMySQL)
	if got := goTypeToSQL(0, "UUID"); got != TypeChar36 {
		t.Fatalf("MySQL UUID: %s", got)
	}
	if got := sqlJSON(); got != TypeMySQLJSON {
		t.Fatalf("MySQL JSON: %s", got)
	}
	SetDialect(DialectSQLite)
	if got := sqlJSON(); got != TypeText {
		t.Fatalf("SQLite JSON: %s", got)
	}
	SetDialect(DialectPostgres)
	if got := goTypeToSQL(0, "Latitude"); got != TypeNumeric129 {
		t.Fatalf("PG Latitude: %s", got)
	}
	SetDialect(DialectMySQL)
	if got := goTypeToSQL(0, "Longitude"); got != TypeDecimal129 {
		t.Fatalf("MySQL Longitude: %s", got)
	}
	SetDialect(DialectSQLite)
	if got := sqlGeoCoord(); got != TypeText {
		t.Fatalf("SQLite geo: %s", got)
	}
	SetDialect(DialectPostgres)
}
