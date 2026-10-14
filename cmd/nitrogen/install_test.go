package main

import "testing"

func TestDriverEnvVarsSQL(t *testing.T) {
	pg := driverEnvVars("postgresql", "secret")
	if pg["NITRO_DB_DRIVER"] != "postgresql" || pg["DATABASE_URL"] == "" || pg["DB_DSN"] == "" {
		t.Fatalf("%v", pg)
	}
	my := driverEnvVars("mysql", "secret")
	if my["NITRO_DB_DRIVER"] != "mysql" || my["DATABASE_URL"] != my["DB_DSN"] {
		t.Fatalf("%v", my)
	}
	sq := driverEnvVars("sqlite", "")
	if sq["NITRO_DB_DRIVER"] != "sqlite" || sq["DB_DSN"] != "file:data/app.db" {
		t.Fatalf("%v", sq)
	}
	if !isSQLDriver("sqlite") || isSQLDriver("redis") {
		t.Fatal("isSQLDriver")
	}
}
