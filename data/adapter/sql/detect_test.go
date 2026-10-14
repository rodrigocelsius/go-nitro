package sqladapter

import "testing"

func TestDetectDriver(t *testing.T) {
	cases := []struct {
		env, dsn string
		want     Driver
	}{
		{"mysql", "user:pass@tcp(127.0.0.1:3306)/app", DriverMySQL},
		{"mariadb", "", DriverMariaDB},
		{"sqlserver", "sqlserver://sa:x@127.0.0.1:1433?database=app", DriverSQLServer},
		{"sqlite", "file:data/app.db", DriverSQLite},
		{"postgresql", "postgres://localhost/app", DriverPostgres},
		{"", "postgres://localhost/app", DriverPostgres},
		{"", "user:x@tcp(127.0.0.1:3306)/app", DriverMySQL},
		{"", "file:foo.db", DriverSQLite},
		{"", "sqlserver://localhost", DriverSQLServer},
	}
	for _, tc := range cases {
		got := detectDriver(tc.env, tc.dsn)
		if got != tc.want {
			t.Errorf("detectDriver(%q,%q)=%q want %q", tc.env, tc.dsn, got, tc.want)
		}
	}
}

func TestStdDriverName(t *testing.T) {
	if stdDriverName(DriverMySQL) != "mysql" {
		t.Fatal(stdDriverName(DriverMySQL))
	}
	if stdDriverName(DriverSQLServer) != "sqlserver" {
		t.Fatal(stdDriverName(DriverSQLServer))
	}
	if stdDriverName(DriverSQLite) != "sqlite" {
		t.Fatal(stdDriverName(DriverSQLite))
	}
	if stdDriverName(DriverPostgres) != "" {
		t.Fatal("postgres uses pgx, not database/sql driver name")
	}
}
