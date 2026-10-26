package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

func setupDB(t *testing.T) *sqladapter.DB {
	t.Helper()
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.MigrateFS(ctx, migrations); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRedis_SharedAcrossReplicas_AndBustedOnWrite(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Setenv("REDIS_ADDR", mr.Addr())
	db := setupDB(t)
	replicaA := nitro.NewTestClient(newApp(db, cacheFromEnv()))
	replicaB := nitro.NewTestClient(newApp(db, cacheFromEnv()))

	resp := replicaA.Post(t, "/drivers", `{"name":"Ada","license":"L-1"}`)
	replicaA.AssertStatus(t, resp, 201)
	id := extractID(t, resp.Body())

	replicaA.AssertBodyContains(t, replicaA.Get(t, "/drivers/"+id), `"name":"Ada"`)
	if len(mr.Keys()) == 0 {
		t.Fatal("expected the read to be cached in Redis")
	}
	// Replica B updates; replica A must not serve the stale cached row.
	replicaB.AssertStatus(t, replicaB.Patch(t, "/drivers/"+id, `{"name":"Grace"}`), 200)
	replicaA.AssertBodyContains(t, replicaA.Get(t, "/drivers/"+id), `"name":"Grace"`)

	replicaA.AssertBodyContains(t, replicaA.Get(t, "/drivers/queries/count"), `"drivers":1`)
}

func TestRedisDown_ReadsStillWork(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Setenv("REDIS_ADDR", mr.Addr())
	db := setupDB(t)
	tc := nitro.NewTestClient(newApp(db, cacheFromEnv()))
	id := extractID(t, tc.Post(t, "/drivers", `{"name":"Ada","license":"L-1"}`).Body())
	mr.Close() // Redis goes away
	tc.AssertBodyContains(t, tc.Get(t, "/drivers/"+id), `"name":"Ada"`)
	tc.AssertStatus(t, tc.Get(t, "/drivers"), 200)
}

func TestNoRedis_InProcessCache(t *testing.T) {
	t.Setenv("REDIS_ADDR", "")
	db := setupDB(t)
	tc := nitro.NewTestClient(newApp(db, cacheFromEnv()))
	id := extractID(t, tc.Post(t, "/drivers", `{"name":"Ada","license":"L-1"}`).Body())
	tc.AssertBodyContains(t, tc.Get(t, "/drivers/"+id), `"name":"Ada"`)
	tc.AssertStatus(t, tc.Patch(t, "/drivers/"+id, `{"name":"Grace"}`), 200)
	tc.AssertBodyContains(t, tc.Get(t, "/drivers/"+id), `"name":"Grace"`) // purged on write
}

func extractID(t *testing.T, body []byte) string {
	t.Helper()
	var d Driver
	if err := json.Unmarshal(body, &d); err != nil || d.ID.IsZero() {
		t.Fatalf("no id in %s", body)
	}
	return d.ID.String()
}
