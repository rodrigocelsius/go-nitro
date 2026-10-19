package sql_test

import (
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	blinqsql "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestCompileRowLocks(t *testing.T) {
	status := field.Col[string]("jobs", "status")
	base := query.FromName("jobs").Where(status.Eq("pending")).Limit(10)
	cases := []struct {
		name string
		q    query.Query
		d    dialect.Dialect
		want string
	}{
		{"pg update skip", base.ForUpdate().SkipLocked(), dialect.Postgres(), " LIMIT 10 FOR UPDATE SKIP LOCKED"},
		{"pg skip implies update", base.SkipLocked(), dialect.Postgres(), "FOR UPDATE SKIP LOCKED"},
		{"pg share nowait", base.ForShare().NoWait(), dialect.Postgres(), "FOR SHARE NOWAIT"},
		{"mysql", base.ForUpdate(), dialect.MySQL(), "FOR UPDATE"},
		{"mssql hint", base.SkipLocked(), dialect.MSSQL(), "WITH (ROWLOCK, UPDLOCK, READPAST)"},
	}
	for _, c := range cases {
		got, err := blinqsql.Compile(c.q.AST(), c.d)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !strings.Contains(got.SQL, c.want) {
			t.Fatalf("%s: %q does not contain %q", c.name, got.SQL, c.want)
		}
	}
	got, _ := blinqsql.Compile(base.ForUpdate().AST(), dialect.SQLite())
	if strings.Contains(got.SQL, "FOR UPDATE") {
		t.Fatalf("sqlite must ignore row locks: %s", got.SQL)
	}
	if strings.Contains(mustSQL(t, base), "FOR ") {
		t.Fatal("no lock unless requested")
	}
}

func TestLockDoesNotMutateBase(t *testing.T) {
	base := query.FromName("jobs")
	_ = base.ForUpdate().SkipLocked()
	if base.AST().Lock != nil {
		t.Fatal("ForUpdate must not mutate the receiver")
	}
	a := base.ForUpdate()
	b := a.SkipLocked()
	if a.AST().Lock.SkipLocked || !b.AST().Lock.SkipLocked {
		t.Fatal("lock clause must be copied on write")
	}
}

func mustSQL(t *testing.T, q query.Query) string {
	t.Helper()
	c, err := blinqsql.Compile(q.AST(), dialect.Postgres())
	if err != nil {
		t.Fatal(err)
	}
	return c.SQL
}
