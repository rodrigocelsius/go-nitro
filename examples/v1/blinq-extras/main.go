// Offline examples of BLinq Phase 10/11 features.
//
//	cd examples/v1/blinq-extras && go run .
//	DATABASE_URL=postgres://... go run . -migrate
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/migrate"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/elastic"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/graphql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/mongo"
	blinqSQL "github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
)

func main() {
	doMigrate := flag.Bool("migrate", false, "apply embed migrations via blinq/migrate + pgx")
	flag.Parse()

	demoMSSQLMerge()
	demoMongoCompile()
	demoElasticCompile()
	demoGraphQLCompile()
	demoMigrateMemory()

	if *doMigrate {
		if err := demoMigratePostgres(context.Background()); err != nil {
			log.Fatal(err)
		}
	}
}

func demoMSSQLMerge() {
	fmt.Println("=== MSSQL MERGE (compile) ===")
	ent := field.Entity{Name: "users"}
	email := field.Col[string]("users", "email")
	name := field.Col[string]("users", "name")
	q := query.UpsertInto(ent).
		Values(name.Set("Ada"), email.Set("ada@x.com")).
		OnConflict(email.Ref()).
		DoUpdate(name.Set("Ada"))
	c, err := blinqSQL.Compile(q.AST(), dialect.MSSQL())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(c.SQL)
	fmt.Println("args:", c.Args)
	fmt.Println()
}

func demoMongoCompile() {
	fmt.Println("=== Mongo CompileFind + CompileLookup ===")
	age := field.Col[int]("users", "age")
	q := query.FromName("users").Where(age.Gte(18)).OrderBy(age.OrderDesc()).Limit(10)
	find, err := mongo.CompileFind(q.AST())
	if err != nil {
		log.Fatal(err)
	}
	b, _ := json.MarshalIndent(find, "", "  ")
	fmt.Println(string(b))

	inc := field.HasMany("orders", field.Entity{Name: "orders"},
		field.Col[int]("users", "id").Ref(),
		field.Col[int]("orders", "user_id").Ref(),
	).ToInclude()
	look, _ := json.MarshalIndent(mongo.CompileLookup(inc), "", "  ")
	fmt.Println(string(look))
	fmt.Println()
}

func demoElasticCompile() {
	fmt.Println("=== Elastic CompileSearch ===")
	age := field.Col[int]("users", "age")
	q := query.FromName("users").Where(age.Gte(18)).Limit(20)
	s, err := elastic.CompileSearch(q.AST())
	if err != nil {
		log.Fatal(err)
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(b))
	fmt.Println()
}

func demoGraphQLCompile() {
	fmt.Println("=== GraphQL CompileQuery ===")
	age := field.Col[int]("users", "age")
	q := query.FromName("users").Where(age.Gte(18)).Limit(5)
	c, err := graphql.CompileQuery(q.AST())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(c.Query)
	b, _ := json.MarshalIndent(c.Variables, "", "  ")
	fmt.Println(string(b))
	fmt.Println()
}

func demoMigrateMemory() {
	fmt.Println("=== migrate Runner (memory) ===")
	db := &memExec{versions: map[int64]time.Time{}}
	src := migrate.MemorySource{
		{Version: 1, Name: "init", UpSQL: "CREATE TABLE demo;", DownSQL: "DROP TABLE demo;"},
	}
	r := &migrate.Runner{DB: db, Source: src}
	ctx := context.Background()
	if err := r.Up(ctx); err != nil {
		log.Fatal(err)
	}
	st, _ := r.Status(ctx)
	fmt.Printf("status: version=%d applied=%v\n", st[0].Version, st[0].Applied)
	_ = r.Down(ctx, 1)
	st, _ = r.Status(ctx)
	fmt.Printf("after down: applied=%v\n", st[0].Applied)
	fmt.Println("meta SQL uses $1:", strings.Contains(strings.Join(db.sqls, "\n"), "$1"))
	fmt.Println()
}

type memExec struct {
	sqls     []string
	versions map[int64]time.Time
}

func (m *memExec) Exec(_ context.Context, sql string, args []any) error {
	m.sqls = append(m.sqls, sql)
	u := strings.ToUpper(strings.TrimSpace(sql))
	if strings.HasPrefix(u, "INSERT INTO") && len(args) >= 1 {
		ver := args[0].(int64)
		at := time.Now().UTC()
		if len(args) > 1 {
			if t, ok := args[1].(time.Time); ok {
				at = t
			}
		}
		m.versions[ver] = at
	}
	if strings.HasPrefix(u, "DELETE FROM") && len(args) >= 1 {
		delete(m.versions, args[0].(int64))
	}
	return nil
}

func (m *memExec) Query(_ context.Context, sql string, _ []any) (migrate.Rows, error) {
	m.sqls = append(m.sqls, sql)
	rows := make([][]any, 0, len(m.versions))
	for v, at := range m.versions {
		rows = append(rows, []any{v, at})
	}
	return &memRows{rows: rows}, nil
}

type memRows struct {
	rows [][]any
	i    int
}

func (r *memRows) Next() bool {
	if r.i >= len(r.rows) {
		return false
	}
	r.i++
	return true
}
func (r *memRows) Scan(dest ...any) error {
	row := r.rows[r.i-1]
	*dest[0].(*int64) = row[0].(int64)
	*dest[1].(*time.Time) = row[1].(time.Time)
	return nil
}
func (r *memRows) Close()     {}
func (r *memRows) Err() error { return nil }

func demoMigratePostgres(ctx context.Context) error {
	db, err := pgxadapter.OpenEnv(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	// inline migration pair
	src := migrate.MemorySource{
		{
			Version: 1,
			Name:    "extras_demo",
			UpSQL:   `CREATE TABLE IF NOT EXISTS extras_demo (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`,
			DownSQL: `DROP TABLE IF EXISTS extras_demo`,
		},
	}
	r := &migrate.Runner{DB: pgxadapter.AdaptMigrate(db.Pool), Source: src}
	if err := r.Up(ctx); err != nil {
		return err
	}
	st, err := r.Status(ctx)
	if err != nil {
		return err
	}
	for _, s := range st {
		fmt.Printf("migrate pg: %d %s applied=%v\n", s.Version, s.Name, s.Applied)
	}
	return nil
}
