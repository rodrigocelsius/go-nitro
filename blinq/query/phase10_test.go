package query_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/expr"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/memory"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider/sql/dialect"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
)

func TestPhase10_AnyAll(t *testing.T) {
	ctx := context.Background()
	db := memory.New("users", []map[string]any{
		{"id": 1, "age": 20},
		{"id": 2, "age": 30},
		{"id": 3, "age": 40},
	})
	age := field.Col[int]("users", "age")

	ok, err := query.FromName("users").Where(age.Gte(18)).Any(ctx, db)
	if err != nil || !ok {
		t.Fatalf("Any: %v %v", ok, err)
	}
	ok, err = query.FromName("users").Where(age.Gt(100)).Any(ctx, db)
	if err != nil || ok {
		t.Fatalf("Any empty: %v %v", ok, err)
	}

	all, err := query.FromName("users").All(ctx, db, age.Gte(18))
	if err != nil || !all {
		t.Fatalf("All: %v %v", all, err)
	}
	all, err = query.FromName("users").All(ctx, db, age.Gte(35))
	if err != nil || all {
		t.Fatalf("All partial: %v %v", all, err)
	}
}

func TestPhase10_AfterBefore(t *testing.T) {
	id := field.Col[int]("users", "id")
	q := query.FromName("users").After(id.Ref(), 10)
	ast := q.AST()
	if ast.Where == nil || len(ast.OrderBy) != 1 {
		t.Fatalf("%+v", ast)
	}
	q2 := query.FromName("users").Before(id.Ref(), 5)
	if q2.AST().Where == nil {
		t.Fatal("before where")
	}
}

func TestPhase10_AliasesAndRaw(t *testing.T) {
	ctx := context.Background()
	db := memory.New("users", []map[string]any{{"id": 1, "name": "Ada"}})
	var out []map[string]any
	if err := query.FromName("users").ToSlice(ctx, db, &out); err != nil || len(out) != 1 {
		t.Fatalf("%v %v", err, out)
	}
	plan, err := query.FromName("users").DryRun(ctx, db)
	if err != nil || plan.Text == "" {
		t.Fatalf("%v %+v", err, plan)
	}
	q := query.FromName("users").RawWhere("1=1")
	if q.AST().Where == nil {
		t.Fatal("raw where")
	}
}

func TestPhase10_CollectTyped(t *testing.T) {
	ctx := context.Background()
	db := memory.NewMulti(map[string][]map[string]any{
		"users":  {{"id": 1, "name": "Ada"}},
		"orders": {{"id": 10, "user_id": 1, "total": 5}},
	})
	type Order struct {
		ID     int `json:"id"`
		UserID int `json:"user_id"`
		Total  int `json:"total"`
	}
	type User struct {
		ID     int     `json:"id"`
		Name   string  `json:"name"`
		Orders []Order `json:"orders"`
	}
	rel := field.HasMany("orders", field.Entity{Name: "orders"},
		field.Col[int]("users", "id").Ref(), field.Col[int]("orders", "user_id").Ref())
	users, err := query.CollectTyped[User](ctx, query.FromName("users").Include(rel), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || len(users[0].Orders) != 1 {
		t.Fatalf("%+v", users)
	}
}

func TestPhase10_CollectTyped_FastPath(t *testing.T) {
	// Sem Includes: scan direto (db tags / snake_case), tipos do driver
	// preservados — sem round-trip JSON.
	ctx := context.Background()
	db := memory.NewMulti(map[string][]map[string]any{
		"events": {
			{"id": 1, "name": "n1", "created_at": time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)},
		},
	})
	type Event struct {
		ID        int       `db:"id" json:"id"`
		Name      string    `db:"name" json:"name"`
		CreatedAt time.Time `db:"created_at" json:"created_at"`
	}
	events, err := query.CollectTyped[Event](ctx, query.FromName("events"), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.ID != 1 || e.Name != "n1" {
		t.Fatalf("%+v", e)
	}
	if e.CreatedAt.IsZero() {
		t.Fatal("time.Time lost in scan")
	}
	if got := e.CreatedAt.UTC(); got.Hour() != 12 {
		t.Fatalf("time value degraded: %v", e.CreatedAt)
	}

	// Campos com db tag divergente do nome também mapeiam.
	db2 := memory.NewMulti(map[string][]map[string]any{
		"things": {{"total_count": 42}},
	})
	type Thing struct {
		TotalCount int `db:"total_count"`
	}
	things, err := query.CollectTyped[Thing](ctx, query.FromName("things"), db2)
	if err != nil {
		t.Fatal(err)
	}
	if things[0].TotalCount != 42 {
		t.Fatalf("db tag not honored: %+v", things[0])
	}
}

func TestPhase4_TenantBulkInsert(t *testing.T) {
	// Regression: .Tenant() + Rows(...) ignorava MultiValues →
	// "row 0 length mismatch" no compile.
	entity := field.Entity{Name: "events"}
	q := query.InsertInto(entity).
		Tenant("t-42").
		Rows(
			[]expr.Assignment{{Field: expr.FieldRef{Column: "name"}, Value: "a"}},
			[]expr.Assignment{{Field: expr.FieldRef{Column: "name"}, Value: "b"}},
			[]expr.Assignment{{Field: expr.FieldRef{Column: "name"}, Value: "c"}},
		)
	c, err := sql.Compile(q.AST(), dialect.Postgres())
	if err != nil {
		t.Fatalf("bulk insert with tenant failed: %v", err)
	}
	got, args := c.SQL, c.Args
	if !strings.Contains(got, `"tenant_id"`) {
		t.Fatalf("tenant column missing: %s", got)
	}
	if len(args) != 6 { // 3 rows x 2 cols
		t.Fatalf("expected 6 args, got %d", len(args))
	}
	for _, want := range []any{"a", "b", "c"} {
		found := false
		for _, arg := range args {
			if arg == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("payload value %v lost", want)
		}
	}
	tenantCount := 0
	for _, arg := range args {
		if arg == "t-42" {
			tenantCount++
		}
	}
	if tenantCount != 3 {
		t.Fatalf("tenant must be in every row, got %d/3", tenantCount)
	}

	// Upsert em lote também.
	up := query.InsertInto(entity).
		Tenant("t-42").
		OnConflict(field.Col[string]("events", "name").Ref()).
		Rows(
			[]expr.Assignment{{Field: expr.FieldRef{Column: "name"}, Value: "x"}},
		)
	if _, err := sql.Compile(up.AST(), dialect.Postgres()); err != nil {
		t.Fatalf("upsert bulk with tenant: %v", err)
	}
}
