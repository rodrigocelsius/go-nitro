//go:build !nitro_nosqlite && !nitro_nomysql && !nitro_nomssql

package sqladapter

import (
	"context"
	"database/sql"
	"slices"
	"testing"
)

// Default builds keep every driver (no behavior change for existing apps).
func TestDefaultBuildRegistersAllDrivers(t *testing.T) {
	for _, d := range []string{"sqlite", "mysql", "sqlserver"} {
		if !slices.Contains(sql.Drivers(), d) {
			t.Fatalf("driver %q missing from default build", d)
		}
	}
	if !IsRetryable(&mysqlErrStub{}) && mysqlRetryable == nil {
		t.Fatal("mysql retry detector must be registered")
	}
	if _, err := Open(context.Background(), "sqlite", "file:tagsdb?mode=memory&cache=shared"); err != nil {
		t.Fatalf("sqlite must open in the default build: %v", err)
	}
}

type mysqlErrStub struct{}

func (*mysqlErrStub) Error() string { return "stub" }
