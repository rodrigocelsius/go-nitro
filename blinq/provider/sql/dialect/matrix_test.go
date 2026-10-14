package dialect_test

import (
	"strings"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func dialects() []dialect.Dialect {
	return []dialect.Dialect{dialect.Postgres(), dialect.SQLite(), dialect.MySQL(), dialect.MSSQL()}
}

func compileWith(q query.Query, d dialect.Dialect) (string, []any, error) {
	c, err := sql.Compile(q.AST(), d)
	if err != nil {
		return "", nil, err
	}
	return c.SQL, c.Args, nil
}

func qi(d dialect.Dialect, name string) string { return d.QuoteIdent(name) }

func TestMatrix_MaxPlaceholders(t *testing.T) {
	want := map[string]int{
		"postgres": 65535,
		"mysql":    65535,
		"sqlite":   999,
		"mssql":    2100,
	}
	for _, d := range dialects() {
		got := d.MaxPlaceholders()
		if got != want[d.Name()] {
			t.Fatalf("%s MaxPlaceholders=%d want %d", d.Name(), got, want[d.Name()])
		}
		if got <= 0 {
			t.Fatalf("%s MaxPlaceholders must be positive", d.Name())
		}
	}
}

func TestMatrix_SelectWhereLimit(t *testing.T) {
	name := field.Col[string]("users", "name")
	q := query.FromName("users").Where(name.Eq("Ada")).Limit(10)

	for _, d := range dialects() {
		got, args, err := compileWith(q, d)
		if err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		ph := d.Placeholder(1)
		want := "SELECT "
		if top := d.SelectTop(uint64Ptr(10), nil); top != "" {
			want += top
		}
		want += "* FROM " + qi(d, "users") + " WHERE " + qi(d, "name") + " = " + ph + d.LimitOffset(uint64Ptr(10), nil)
		if got != want {
			t.Fatalf("%s SQL:\ngot:  %q\nwant: %q", d.Name(), got, want)
		}
		if len(args) != 1 || args[0] != "Ada" {
			t.Fatalf("%s args: %v", d.Name(), args)
		}
	}
}

func uint64Ptr(n uint64) *uint64 { return &n }

func TestMatrix_ILike(t *testing.T) {
	name := field.Col[string]("users", "name")
	q := query.FromName("users").Where(name.ILike("a%"))

	for _, d := range dialects() {
		got, _, err := compileWith(q, d)
		if err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		col := qi(d, "name")
		ph := d.Placeholder(1)
		want := "SELECT * FROM " + qi(d, "users") + " WHERE " + d.RenderILike(col, ph) + ` ESCAPE '\'`
		if got != want {
			t.Fatalf("%s SQL:\ngot:  %q\nwant: %q", d.Name(), got, want)
		}
	}
}

func TestMatrix_SoftDelete(t *testing.T) {
	ent := field.Entity{Name: "users", SoftColumn: "deleted_at"}
	id := field.Col[int]("users", "id")
	q := query.DeleteFrom(ent).Soft().Where(id.Eq(1))

	for _, d := range dialects() {
		got, _, err := compileWith(q, d)
		if err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		want := "UPDATE " + qi(d, "users") + " SET " + qi(d, "deleted_at") + " = " + d.SoftDeleteNow() +
			" WHERE " + qi(d, "id") + " = " + d.Placeholder(1)
		if got != want {
			t.Fatalf("%s SQL:\ngot:  %q\nwant: %q", d.Name(), got, want)
		}
	}
}

func TestMatrix_InsertReturning(t *testing.T) {
	ent := field.Entity{Name: "users"}
	name := field.Col[string]("users", "name")
	q := query.InsertInto(ent).Values(name.Set("X"))

	for _, d := range dialects() {
		got, _, err := compileWith(q, d)
		if err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		hasRet := strings.Contains(got, "RETURNING")
		if d.ReturningSupported() && !hasRet {
			t.Fatalf("%s: expected RETURNING in %q", d.Name(), got)
		}
		if !d.ReturningSupported() && hasRet {
			t.Fatalf("%s: unexpected RETURNING in %q", d.Name(), got)
		}
	}
}

func TestMatrix_UpsertDoUpdate(t *testing.T) {
	ent := field.Entity{Name: "users"}
	email := field.Col[string]("users", "email")
	name := field.Col[string]("users", "name")
	q := query.UpsertInto(ent).
		OnConflict(email.Ref()).
		Values(name.Set("Ada"), email.Set("a@x.com")).
		DoUpdate(name.Set(""))

	for _, d := range dialects() {
		got, _, err := compileWith(q, d)
		if err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		switch d.Name() {
		case "mssql":
			want := "MERGE [users] AS [target] USING (VALUES (@p1, @p2)) AS [source] ([name], [email]) " +
				"ON [target].[email] = [source].[email] " +
				"WHEN MATCHED THEN UPDATE SET [name] = [source].[name] " +
				"WHEN NOT MATCHED THEN INSERT ([name], [email]) VALUES ([source].[name], [source].[email]);"
			if got != want {
				t.Fatalf("mssql:\n got %s\nwant %s", got, want)
			}
		case "postgres":
			if !strings.Contains(got, "INSERT INTO "+qi(d, "users")) {
				t.Fatalf("%s: missing quoted table: %s", d.Name(), got)
			}
			if !strings.Contains(got, `ON CONFLICT ("email") DO UPDATE SET "name" = EXCLUDED."name"`) {
				t.Fatalf("postgres: %s", got)
			}
		case "sqlite":
			if !strings.Contains(got, "INSERT INTO "+qi(d, "users")) {
				t.Fatalf("%s: missing quoted table: %s", d.Name(), got)
			}
			if !strings.Contains(got, `ON CONFLICT ("email") DO UPDATE SET "name" = excluded."name"`) {
				t.Fatalf("sqlite: %s", got)
			}
		case "mysql":
			if !strings.Contains(got, "INSERT INTO "+qi(d, "users")) {
				t.Fatalf("%s: missing quoted table: %s", d.Name(), got)
			}
			if !strings.Contains(got, "ON DUPLICATE KEY UPDATE `name` = VALUES(`name`)") {
				t.Fatalf("mysql: %s", got)
			}
		}
	}
}

func TestMatrix_UpsertDoNothing(t *testing.T) {
	ent := field.Entity{Name: "users"}
	email := field.Col[string]("users", "email")
	name := field.Col[string]("users", "name")
	q := query.UpsertInto(ent).
		OnConflict(email.Ref()).
		DoNothing().
		Values(email.Set("a@b.c"), name.Set("X"))

	for _, d := range dialects() {
		got, _, err := compileWith(q, d)
		switch d.Name() {
		case "mysql":
			if err == nil {
				t.Fatalf("%s: expected DoNothing/upsert error, got %q", d.Name(), got)
			}
		case "mssql":
			want := "MERGE [users] AS [target] USING (VALUES (@p1, @p2)) AS [source] ([email], [name]) " +
				"ON [target].[email] = [source].[email] " +
				"WHEN NOT MATCHED THEN INSERT ([email], [name]) VALUES ([source].[email], [source].[name]);"
			if err != nil || got != want {
				t.Fatalf("mssql: got %q err=%v", got, err)
			}
			if strings.Contains(got, "WHEN MATCHED") {
				t.Fatalf("mssql DoNothing must omit WHEN MATCHED: %s", got)
			}
		case "postgres":
			want := `INSERT INTO "users" ("email", "name") VALUES ($1, $2) ON CONFLICT ("email") DO NOTHING RETURNING *`
			if err != nil || got != want {
				t.Fatalf("postgres: got %q err=%v", got, err)
			}
		case "sqlite":
			want := `INSERT INTO "users" ("email", "name") VALUES (?, ?) ON CONFLICT ("email") DO NOTHING`
			if err != nil || got != want {
				t.Fatalf("sqlite: got %q err=%v", got, err)
			}
		}
	}
}

func TestMatrix_Inc(t *testing.T) {
	ent := field.Entity{Name: "users"}
	login := field.Col[int]("users", "login_count")
	id := field.Col[int]("users", "id")
	q := query.Update(ent).Set(login.Inc(1)).Where(id.Eq(9))

	for _, d := range dialects() {
		got, args, err := compileWith(q, d)
		if err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		ph1, ph2 := d.Placeholder(1), d.Placeholder(2)
		col := qi(d, "login_count")
		base := "UPDATE " + qi(d, "users") + " SET " + col + " = " + col + " + " + ph1 +
			" WHERE " + qi(d, "id") + " = " + ph2
		if d.ReturningSupported() {
			base += " RETURNING *"
		}
		if got != base {
			t.Fatalf("%s SQL:\ngot:  %q\nwant: %q", d.Name(), got, base)
		}
		if len(args) != 2 {
			t.Fatalf("%s args: %v", d.Name(), args)
		}
	}
}

func TestMatrix_Join(t *testing.T) {
	users := field.Entity{Name: "users"}
	orders := field.Entity{Name: "orders"}
	q := query.From(users).
		Join(orders, field.Col[int]("users", "id").Ref(), field.Col[int]("orders", "user_id").Ref()).
		Select(field.Col[string]("users", "name").Ref())

	for _, d := range dialects() {
		got, _, err := compileWith(q, d)
		if err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		want := "SELECT " + qi(d, "users") + "." + qi(d, "name") + " FROM " + qi(d, "users") +
			" INNER JOIN " + qi(d, "orders") + " ON " + qi(d, "users") + "." + qi(d, "id") +
			" = " + qi(d, "orders") + "." + qi(d, "user_id")
		if got != want {
			t.Fatalf("%s SQL:\ngot:  %q\nwant: %q", d.Name(), got, want)
		}
	}
}

func TestMatrix_MSSQL_Smoke(t *testing.T) {
	d := dialect.MSSQL()
	if d.Placeholder(1) != "@p1" {
		t.Fatal(d.Placeholder(1))
	}
	if d.QuoteIdent("users") != "[users]" {
		t.Fatal(d.QuoteIdent("users"))
	}
	lim := uint64(5)
	off := uint64(10)
	lo := d.LimitOffset(&lim, &off)
	if !strings.Contains(lo, "OFFSET 10 ROWS") || !strings.Contains(lo, "FETCH NEXT 5 ROWS ONLY") {
		t.Fatal(lo)
	}
	if top := d.SelectTop(&lim, nil); top != "TOP (5) " {
		t.Fatalf("SelectTop: %q", top)
	}
	if d.LimitOffset(&lim, nil) != "" {
		t.Fatal("limit-only should use TOP, not OFFSET/FETCH")
	}
	if d.ReturningSupported() {
		t.Fatal("mssql returning")
	}
	if d.SoftDeleteNow() != "GETDATE()" {
		t.Fatal(d.SoftDeleteNow())
	}
}

func TestQuoteIdent(t *testing.T) {
	if dialect.Postgres().QuoteIdent("users") != `"users"` {
		t.Fatal("postgres quote")
	}
	if dialect.SQLite().QuoteIdent("users") != `"users"` {
		t.Fatal("sqlite quote")
	}
	if dialect.MySQL().QuoteIdent("users") != "`users`" {
		t.Fatal("mysql quote")
	}
	if dialect.MSSQL().QuoteIdent("users") != "[users]" {
		t.Fatal("mssql quote")
	}
	if dialect.Postgres().QuoteIdent(`us"ers`) != `"us""ers"` {
		t.Fatal("postgres escape")
	}
	if dialect.MySQL().QuoteIdent("us`ers") != "`us``ers`" {
		t.Fatal("mysql escape")
	}
	if dialect.MSSQL().QuoteIdent("us]ers") != "[us]]ers]" {
		t.Fatal("mssql escape")
	}
}

func TestUpsertDefaultSets(t *testing.T) {
	ent := field.Entity{Name: "users"}
	email := field.Col[string]("users", "email")
	name := field.Col[string]("users", "name")
	age := field.Col[int]("users", "age")
	q := query.UpsertInto(ent).
		OnConflict(email.Ref()).
		Values(name.Set("A"), email.Set("e"), age.Set(1)).
		DoUpdate()

	ast := q.AST()
	if ast.Conflict == nil || ast.Conflict.Action != expr.ConflictDoUpdate {
		t.Fatal("expected DoUpdate")
	}

	pg, _, err := compileWith(q, dialect.Postgres())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pg, `"name" = EXCLUDED."name"`) || !strings.Contains(pg, `"age" = EXCLUDED."age"`) {
		t.Fatalf("postgres default sets: %s", pg)
	}
	my, _, err := compileWith(q, dialect.MySQL())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(my, "VALUES(`name`)") || !strings.Contains(my, "VALUES(`age`)") {
		t.Fatalf("mysql default sets: %s", my)
	}
}
