package sql_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/cache"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/postgres"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestPhase10_QuoteIdent(t *testing.T) {
	got, _ := compile(query.FromName("users").Where(User.Name.Eq("x")))
	want := `SELECT * FROM "users" WHERE "name" = $1`
	if got != want {
		t.Fatalf("%q", got)
	}
}

func TestPhase10_OuterRefAndRaw(t *testing.T) {
	parentID := field.Col[int]("users", "id")
	fk := field.Col[int]("orders", "user_id")
	sub := query.FromName("orders").Where(fk.EqOuter(parentID.Ref()))
	q := query.FromName("users").Where(query.Exists(sub))
	got, args := compile(q)
	if !strings.Contains(got, `"orders"."user_id" = "users"."id"`) {
		t.Fatalf("%s", got)
	}
	if len(args) != 0 {
		t.Fatalf("outer ref must not use placeholders: %v", args)
	}

	got2, args2 := compile(query.FromName("users").RawWhere("age > $1", 18))
	if !strings.Contains(got2, "(age > $1)") {
		t.Fatalf("%s", got2)
	}
	if len(args2) != 1 || args2[0] != 18 {
		t.Fatalf("%v", args2)
	}
}

func TestPhase10_JSON(t *testing.T) {
	meta := field.Col[any]("users", "meta")
	q := query.FromName("users").Where(meta.JSONContains(map[string]any{"a": 1}))
	got, args := compile(q)
	if !strings.Contains(got, `@> $1::jsonb`) {
		t.Fatalf("%s", got)
	}
	if len(args) != 1 {
		t.Fatalf("%v", args)
	}

	q2 := query.FromName("users").Where(meta.JSONPathEq("role", "admin"))
	got2, _ := compile(q2)
	if !strings.Contains(got2, `#>> '{role}'`) {
		t.Fatalf("%s", got2)
	}

	_, err := sql.Compile(q.AST(), dialect.SQLite())
	if !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("sqlite JSONContains: %v", err)
	}
}

func TestPhase10_JSONPathInjection(t *testing.T) {
	meta := field.Col[any]("users", "meta")

	// Hostile path must be escaped inside the SQL literal, never break out.
	hostile := `x'} = 'pwned' OR '1'='1`
	got, _ := compile(query.FromName("users").Where(meta.JSONPathEq(hostile, 1)))
	if strings.Contains(got, `'}`) && !strings.Contains(got, `''}`) {
		t.Fatalf("quote breakout in postgres path: %s", got)
	}
	if strings.Count(got, "#>>") != 1 || !strings.HasSuffix(strings.Split(got, "#>>")[0], `"meta" `) {
		t.Fatalf("unexpected postgres shape: %s", got)
	}
	if strings.Contains(got, "= $1 = $") {
		t.Fatalf("injected comparison in postgres path: %s", got)
	}

	multiSeg := query.FromName("users").Where(meta.JSONPathEq("a.b.c", 1))
	gotMS, _ := compile(multiSeg)
	if !strings.Contains(gotMS, `#>> '{a,b,c}'`) {
		t.Fatalf("%s", gotMS)
	}

	for _, name := range []string{"mysql", "sqlite"} {
		var d dialect.Dialect
		var fn string
		switch name {
		case "mysql":
			d = dialect.MySQL()
			fn = "JSON_EXTRACT"
		case "sqlite":
			d = dialect.SQLite()
			fn = "json_extract"
		}
		gotM, err := sql.Compile(query.FromName("users").Where(meta.JSONPathEq(hostile, 1)).AST(), d)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if strings.Contains(gotM.SQL, "',") || strings.Contains(gotM.SQL, "')") && strings.Count(gotM.SQL, fn) != 1 {
			t.Fatalf("%s: quote breakout: %s", name, gotM.SQL)
		}
		// The hostile single quotes must appear doubled.
		if !strings.Contains(gotM.SQL, `$."x''`) {
			t.Fatalf("%s: expected doubled quotes: %s", name, gotM.SQL)
		}

		gotPlain, _ := sql.Compile(query.FromName("users").Where(meta.JSONPathEq("a.b", 1)).AST(), d)
		if !strings.Contains(gotPlain.SQL, "'$.a.b')") {
			t.Fatalf("%s: plain path changed: %s", name, gotPlain.SQL)
		}

		oddKey := query.FromName("users").Where(meta.JSONPathEq(`we ird"key`, 1))
		gotOdd, err := sql.Compile(oddKey.AST(), d)
		if err != nil {
			t.Fatalf("%s: odd key: %v", name, err)
		}
		wantQuoted := `$."we ird""key"`
		if !strings.Contains(gotOdd.SQL, wantQuoted) {
			t.Fatalf("%s: quoted segment missing %q in: %s", name, wantQuoted, gotOdd.SQL)
		}
	}

	for _, bad := range []string{"", "a..b", ".a", "a.", `a\b`} {
		_, err := sql.Compile(query.FromName("users").Where(meta.JSONPathEq(bad, 1)).AST(), dialect.Postgres())
		if err == nil {
			t.Fatalf("expected error for path %q", bad)
		}
	}
}

func TestPhase10_SetExpr(t *testing.T) {
	name := field.Col[string]("users", "name")
	id := field.Col[int]("users", "id")
	q := query.Update(field.Entity{Name: "users"}).
		Set(name.SetExpr(expr.Expr{SQL: "UPPER(" + `"` + "name" + `"` + ")"})).
		Where(id.Eq(1))
	got, _ := compile(q)
	if !strings.Contains(got, `SET "name" = UPPER("name")`) {
		t.Fatalf("%s", got)
	}
}

func TestPhase10_SuggestIndexes(t *testing.T) {
	age := field.Col[int]("users", "age")
	q := query.FromName("users").
		Where(age.Gte(18)).
		OrderBy(age.OrderAsc()).
		Join(field.Entity{Name: "orders"}, field.Col[int]("users", "id").Ref(), field.Col[int]("orders", "user_id").Ref())
	sugs := sql.SuggestIndexes(q.AST())
	if len(sugs) < 3 {
		t.Fatalf("%+v", sugs)
	}
	hasAge, hasJoin := false, false
	for _, s := range sugs {
		if s.Table == "users" {
			for _, c := range s.Columns {
				if c == "age" {
					hasAge = true
				}
			}
		}
		if s.Table == "orders" || (s.Table == "users" && len(s.Columns) > 0) {
			hasJoin = true
		}
	}
	if !hasAge {
		t.Fatalf("expected age index suggestion: %+v", sugs)
	}
	_ = hasJoin
}

func TestPhase10_WithCache(t *testing.T) {
	db := &cacheStubDB{rows: []map[string]any{{"id": 1}}}
	c := cache.New()
	p := sql.New(db, dialect.Postgres(), sql.WithCache(c))
	ctx := context.Background()
	q := query.FromName("users").Cache(time.Minute)
	res, err := q.Execute(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := res.Count()
	if n != 1 || db.hits != 1 {
		t.Fatalf("first: n=%d hits=%d", n, db.hits)
	}
	res2, err := q.Execute(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	n2, _ := res2.Count()
	if n2 != 1 || db.hits != 1 {
		t.Fatalf("cache hit should skip db: n=%d hits=%d", n2, db.hits)
	}
}

func TestPhase10_PostgresHelper(t *testing.T) {
	p := postgres.New(&stubDB{})
	plan, err := p.Explain(context.Background(), query.FromName("users").AST())
	if err != nil || !strings.Contains(plan.Text, `"users"`) {
		t.Fatalf("%v %q", err, plan.Text)
	}
}

type cacheStubDB struct {
	rows []map[string]any
	hits int
}

func (c *cacheStubDB) QueryMaps(context.Context, string, []any) ([]map[string]any, error) {
	c.hits++
	return c.rows, nil
}
func (c *cacheStubDB) Exec(context.Context, string, string, []any) (int64, error) { return 0, nil }
func (c *cacheStubDB) QueryRowScan(context.Context, string, []any, ...any) error  { return nil }
