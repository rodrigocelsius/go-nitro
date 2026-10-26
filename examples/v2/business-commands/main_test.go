package main

import (
	"context"
	"encoding/json"
	"testing"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

const secret = "test-secret-at-least-32-characters-long"

func setup(t *testing.T) (*nitro.TestClient, *nitro.Auth, *sqladapter.DB) {
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
	auth := nitro.NewAuth(nitro.AuthConfig{JWTSecret: secret}).WithMemoryStore().WithRateLimit(0, 0)
	return nitro.NewTestClient(newApp(db, auth)), auth, db
}

func token(t *testing.T, auth *nitro.Auth, id string, roles ...string) string {
	t.Helper()
	tok, err := auth.Provider().GenerateAccessToken(context.Background(), &nitro.User{ID: id, Email: id + "@shop.co", Roles: roles})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func place(t *testing.T, tc *nitro.TestClient, tok string, total float64) string {
	t.Helper()
	resp := tc.Bearer(tok).PostJSON(t, "/orders/place", map[string]any{"customer_id": "c1", "total": total})
	tc.AssertStatus(t, resp, 201)
	var o Order
	_ = json.Unmarshal(resp.Body(), &o)
	return o.ID.String()
}

func TestOrderLifecycle(t *testing.T) {
	tc, auth, db := setup(t)
	user := token(t, auth, "u1")
	id := place(t, tc, user, 250)

	tc.AssertStatus(t, tc.Post(t, "/orders/"+id+"/submit", ""), 401) // JWT by default
	tc.AssertBodyContains(t, tc.Bearer(user).Post(t, "/orders/"+id+"/submit", ""), `"status":"pending"`)
	tc.AssertBodyContains(t, tc.GetBearer(t, "/orders/queries/pending", user), id)

	tc.AssertStatus(t, tc.Bearer(user).PostJSON(t, "/orders/"+id+"/cancel", map[string]string{"reason": "x"}), 422) // validated body
	tc.AssertBodyContains(t, tc.Bearer(user).PostJSON(t, "/orders/"+id+"/cancel", map[string]string{"reason": "changed mind"}), `"status":"cancelled"`)
	tc.AssertStatus(t, tc.Bearer(user).Post(t, "/orders/"+id+"/submit", ""), 409) // Rule

	n, _ := sqladapter.StoreFor[OrderEvent](db).CountAll(context.Background())
	if n != 2 {
		t.Fatalf("audit events = %d, want 2 (submit + cancel)", n)
	}
	tc.AssertStatus(t, tc.Bearer(user).PostJSON(t, "/orders", map[string]any{"customer_id": "c"}), 405) // no plain create
}

func TestApproveNeedsManagerOnlyForHighValue(t *testing.T) {
	tc, auth, _ := setup(t)
	user, manager := token(t, auth, "u1"), token(t, auth, "m1", "manager")

	low := place(t, tc, user, 500)
	tc.Bearer(user).Post(t, "/orders/"+low+"/submit", "")
	tc.AssertStatus(t, tc.Bearer(user).Post(t, "/orders/"+low+"/approve", ""), 200)

	high := place(t, tc, user, 50_000)
	tc.Bearer(user).Post(t, "/orders/"+high+"/submit", "")
	tc.AssertStatus(t, tc.Bearer(user).Post(t, "/orders/"+high+"/approve", ""), 403)
	tc.AssertBodyContains(t, tc.Bearer(manager).Post(t, "/orders/"+high+"/approve", ""), `"status":"approved"`)
}
