package migrate_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/migrate"
)

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
	for i, d := range dest {
		switch p := d.(type) {
		case *int64:
			*p = row[i].(int64)
		case *time.Time:
			*p = row[i].(time.Time)
		case *any:
			*p = row[i]
		default:
			return fmt.Errorf("unsupported scan %T", d)
		}
	}
	return nil
}

func (r *memRows) Close()     {}
func (r *memRows) Err() error { return nil }

type fakeExec struct {
	sqls     []string
	versions map[int64]time.Time
}

func (f *fakeExec) Exec(_ context.Context, sql string, args []any) error {
	f.sqls = append(f.sqls, sql)
	u := strings.ToUpper(strings.TrimSpace(sql))
	if strings.HasPrefix(u, "INSERT INTO") && len(args) >= 1 {
		ver := args[0].(int64)
		at := time.Now().UTC()
		if len(args) > 1 {
			if t, ok := args[1].(time.Time); ok {
				at = t
			}
		}
		f.versions[ver] = at
	}
	if strings.HasPrefix(u, "DELETE FROM") && len(args) >= 1 {
		delete(f.versions, args[0].(int64))
	}
	return nil
}

func (f *fakeExec) Query(_ context.Context, sql string, _ []any) (migrate.Rows, error) {
	f.sqls = append(f.sqls, sql)
	rows := make([][]any, 0, len(f.versions))
	for v, at := range f.versions {
		rows = append(rows, []any{v, at})
	}
	return &memRows{rows: rows}, nil
}

func TestRunnerUpDownStatus(t *testing.T) {
	src := migrate.MemorySource{
		{Version: 1, Name: "a", UpSQL: "CREATE TABLE a;", DownSQL: "DROP TABLE a;"},
		{Version: 2, Name: "b", UpSQL: "CREATE TABLE b;", DownSQL: "DROP TABLE b;"},
	}
	db := &fakeExec{versions: map[int64]time.Time{}}
	r := &migrate.Runner{DB: db, Source: src}

	ctx := context.Background()
	if err := r.Up(ctx); err != nil {
		t.Fatal(err)
	}
	st, err := r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 2 || !st[0].Applied || !st[1].Applied {
		t.Fatalf("%#v", st)
	}
	joined := strings.Join(db.sqls, "\n")
	if !strings.Contains(joined, "CREATE TABLE a;") || !strings.Contains(joined, "CREATE TABLE b;") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "blinq_schema_migrations") {
		t.Fatal(joined)
	}
	// BindPostgres default: $1 / $2
	if !strings.Contains(joined, "VALUES ($1, $2)") {
		t.Fatalf("expected $N placeholders, got:\n%s", joined)
	}

	if err := r.Down(ctx, 1); err != nil {
		t.Fatal(err)
	}
	st, err = r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st[0].Applied || st[1].Applied {
		t.Fatalf("after down1: %#v", st)
	}
	if !strings.Contains(strings.Join(db.sqls, "\n"), "DROP TABLE b;") {
		t.Fatal(db.sqls)
	}
	if !strings.Contains(strings.Join(db.sqls, "\n"), "WHERE version = $1") {
		t.Fatalf("down delete should use $1: %v", db.sqls)
	}
}

func TestRunnerBindQMark(t *testing.T) {
	db := &fakeExec{versions: map[int64]time.Time{}}
	r := &migrate.Runner{
		DB:     db,
		Source: migrate.MemorySource{{Version: 1, Name: "a", UpSQL: "SELECT 1;", DownSQL: "SELECT 0;"}},
		Bind:   migrate.BindQMark,
	}
	if err := r.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(db.sqls, "\n")
	if !strings.Contains(joined, "VALUES (?, ?)") {
		t.Fatal(joined)
	}
}

func TestGooseFSSource(t *testing.T) {
	content := `-- +goose Up
CREATE TABLE t;
-- +goose Down
DROP TABLE t;
`
	fsys := fstest.MapFS{
		"000003_create_t.sql": {Data: []byte(content)},
	}
	migs, err := migrate.GooseFSSource{FS: fsys}.Migrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 1 || migs[0].Version != 3 || !strings.Contains(migs[0].UpSQL, "CREATE TABLE t") {
		t.Fatalf("%#v", migs)
	}
	if !strings.Contains(migs[0].DownSQL, "DROP TABLE t") {
		t.Fatalf("%#v", migs[0].DownSQL)
	}
}

func TestFSSource(t *testing.T) {
	fsys := fstest.MapFS{
		"000001_create_users.up.sql":   {Data: []byte("CREATE TABLE users;")},
		"000001_create_users.down.sql": {Data: []byte("DROP TABLE users;")},
		"000002_add_email.up.sql":      {Data: []byte("ALTER TABLE users ADD email;")},
		"000002_add_email.down.sql":    {Data: []byte("ALTER TABLE users DROP email;")},
	}
	migs, err := migrate.FSSource{FS: fsys}.Migrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 2 {
		t.Fatalf("%#v", migs)
	}
	if migs[0].Version != 1 || migs[0].Name != "create_users" || migs[0].UpSQL == "" {
		t.Fatalf("%#v", migs[0])
	}
	if migs[1].Version != 2 || migs[1].Name != "add_email" {
		t.Fatalf("%#v", migs[1])
	}
}
