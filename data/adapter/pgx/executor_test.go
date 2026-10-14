package pgx_test

import (
	"context"
	"os"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	"github.com/rodrigocelsius/go-nitro/blinq/v2/query"
	pgxadapter "github.com/rodrigocelsius/go-nitro/data/adapter/pgx/v2"
)

func skipIfNoDB(t *testing.T) {
	t.Helper()
	if os.Getenv("PG_URL") == "" {
		t.Skip("PG_URL not set; skipping integration tests")
	}
}

func TestPool_Connect(t *testing.T) {
	skipIfNoDB(t)

	ctx := context.Background()
	pool, err := pgxadapter.New(ctx, os.Getenv("PG_URL"))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping: %v", err)
	}
}

func TestDataAPI_Select(t *testing.T) {
	skipIfNoDB(t)

	ctx := context.Background()
	pool, err := pgxadapter.New(ctx, os.Getenv("PG_URL"))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	api := pgxadapter.NewDataAPI(pool)
	rows, err := api.Select(ctx, query.FromName("users").Limit(10).AST())
	if err != nil {
		t.Fatalf("failed to select: %v", err)
	}
	t.Logf("selected %d rows", len(rows))
}

func TestDataAPI_Count(t *testing.T) {
	skipIfNoDB(t)

	ctx := context.Background()
	pool, err := pgxadapter.New(ctx, os.Getenv("PG_URL"))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	api := pgxadapter.NewDataAPI(pool)
	total, err := api.Count(ctx, query.FromName("users").AST())
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	t.Logf("total users: %d", total)
}

func TestDataAPI_InsertAndDelete(t *testing.T) {
	skipIfNoDB(t)

	ctx := context.Background()
	pool, err := pgxadapter.New(ctx, os.Getenv("PG_URL"))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	api := pgxadapter.NewDataAPI(pool)
	ent := field.Entity{Name: "users"}
	name := field.Col[string]("users", "name")
	email := field.Col[string]("users", "email")

	err = api.Insert(ctx, query.InsertInto(ent).Values(
		name.Set("Test User"),
		email.Set("test@example.com"),
	).AST())
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	_, err = api.Delete(ctx, query.DeleteFrom(ent).Where(email.Eq("test@example.com")).AST())
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}
}

func TestDataAPI_Update(t *testing.T) {
	skipIfNoDB(t)

	ctx := context.Background()
	pool, err := pgxadapter.New(ctx, os.Getenv("PG_URL"))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	api := pgxadapter.NewDataAPI(pool)
	ent := field.Entity{Name: "users"}
	name := field.Col[string]("users", "name")
	email := field.Col[string]("users", "email")

	err = api.Update(ctx, query.Update(ent).
		Set(name.Set("Updated Name")).
		Where(email.Eq("test@example.com")).AST())
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}
}

func TestDataAPI_SelectWithCount(t *testing.T) {
	skipIfNoDB(t)

	ctx := context.Background()
	pool, err := pgxadapter.New(ctx, os.Getenv("PG_URL"))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	api := pgxadapter.NewDataAPI(pool)
	sel := query.FromName("users").Limit(10).Offset(0).AST()
	cnt := query.FromName("users").AST()

	rows, total, err := api.SelectWithCount(ctx, sel, cnt)
	if err != nil {
		t.Fatalf("failed to select with count: %v", err)
	}
	t.Logf("selected %d rows, total %d", len(rows), total)
}
