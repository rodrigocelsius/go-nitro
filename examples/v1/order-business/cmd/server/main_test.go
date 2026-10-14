package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/handler"
	"github.com/rodrigocelsius/go-nitro/examples/v1/order-business/v2/internal/model"
)

const schemaSQLite = `
CREATE TABLE IF NOT EXISTS orders (
    id         TEXT PRIMARY KEY NOT NULL,
    customer_id TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'draft',
    total      REAL NOT NULL DEFAULT 0,
    note       TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);
CREATE TABLE IF NOT EXISTS invoices (
    id         TEXT PRIMARY KEY NOT NULL,
    order_id   TEXT NOT NULL,
    amount     REAL NOT NULL DEFAULT 0,
    status     TEXT NOT NULL DEFAULT 'open',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);`

// testApp wires the same Register(...) used in production, but against an
// isolated in-memory SQLite DB (sqladapter.OpenTest) — no Docker needed.
func testApp(t *testing.T) (*nitro.App, *nitro.Auth) {
	t.Helper()
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatalf("OpenTest: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Gateway().Exec(ctx, "schema", schemaSQLite, nil); err != nil {
		t.Fatalf("schema: %v", err)
	}

	auth := nitro.NewAuth(nitro.AuthConfig{
		JWTSecret:      "test-secret-at-least-32-characters-long",
		AccessTokenTTL: time.Hour,
	}).WithMemoryStore()

	app := nitro.New()
	app.AuthRoutes("/auth", auth)

	store := sqladapter.StoreOf[model.Order](db, "orders")
	handler.Register(app, db, store, nitro.JWTMiddleware(auth.Provider()))
	return app, auth
}

func registerUser(t *testing.T, tc *nitro.TestClient, email string) string {
	t.Helper()
	resp := tc.Post(t, "/auth/register", `{"email":"`+email+`","password":"12345678"}`)
	tc.AssertStatus(t, resp, 201)
	return extractField(t, resp.Body(), "access_token")
}

func managerToken(t *testing.T, auth *nitro.Auth) string {
	t.Helper()
	tok, err := auth.Provider().GenerateAccessToken(context.Background(), &nitro.User{
		ID: "mgr-1", Email: "mgr@shop.co", Roles: []string{"manager"},
	})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	return tok
}

func extractField(t *testing.T, body []byte, key string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	v, _ := m[key].(string)
	if v == "" {
		t.Fatalf("missing %s: %s", key, body)
	}
	return v
}

func placeOrder(t *testing.T, tc *nitro.TestClient, token string, total float64) string {
	t.Helper()
	resp := tc.Bearer(token).Post(t, "/orders/place", `{"customer_id":"cus_1","total":`+jsonFloat(total)+`}`)
	tc.AssertStatus(t, resp, 201)
	return extractField(t, resp.Body(), "id")
}

func jsonFloat(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func TestOrderBusiness_PlaceSubmitCancel(t *testing.T) {
	app, _ := testApp(t)
	tc := nitro.NewTestClient(app)
	token := registerUser(t, tc, "user1@shop.co")

	id := placeOrder(t, tc, token, 250)

	submit := tc.Bearer(token).Post(t, "/orders/"+id+"/submit", "")
	tc.AssertStatus(t, submit, 200)
	tc.AssertBodyContains(t, submit, `"status":"pending"`)

	cancel := tc.Bearer(token).Post(t, "/orders/"+id+"/cancel", `{"reason":"customer changed mind"}`)
	tc.AssertStatus(t, cancel, 200)
	tc.AssertBodyContains(t, cancel, `"status":"cancelled"`)

	// Rule: cancelled orders reject further Commands.
	again := tc.Bearer(token).Post(t, "/orders/"+id+"/submit", "")
	if again.StatusCode() == 200 {
		t.Fatal("expected conflict submitting a cancelled order")
	}
}

func TestOrderBusiness_ApproveRequiresManagerOnlyWhenHighValue(t *testing.T) {
	app, auth := testApp(t)
	tc := nitro.NewTestClient(app)
	userToken := registerUser(t, tc, "user2@shop.co")

	// Low value: any authenticated user can approve.
	lowID := placeOrder(t, tc, userToken, 500)
	tc.AssertStatus(t, tc.Bearer(userToken).Post(t, "/orders/"+lowID+"/submit", ""), 200)
	lowApprove := tc.Bearer(userToken).Post(t, "/orders/"+lowID+"/approve", `{"reason":"looks good"}`)
	tc.AssertStatus(t, lowApprove, 200)

	// High value: requires the manager role.
	highID := placeOrder(t, tc, userToken, 50000)
	tc.AssertStatus(t, tc.Bearer(userToken).Post(t, "/orders/"+highID+"/submit", ""), 200)

	denied := tc.Bearer(userToken).Post(t, "/orders/"+highID+"/approve", `{"reason":"needs review"}`)
	if denied.StatusCode() == 200 {
		t.Fatal("expected 403 approving a high value order without manager role")
	}

	mgrToken := managerToken(t, auth)
	approved := tc.Bearer(mgrToken).Post(t, "/orders/"+highID+"/approve", `{"reason":"reviewed by manager"}`)
	tc.AssertStatus(t, approved, 200)
	tc.AssertBodyContains(t, approved, `"status":"approved"`)
}

func TestOrderBusiness_PendingQuery(t *testing.T) {
	app, _ := testApp(t)
	tc := nitro.NewTestClient(app)
	token := registerUser(t, tc, "user3@shop.co")

	id := placeOrder(t, tc, token, 300)
	tc.AssertStatus(t, tc.Bearer(token).Post(t, "/orders/"+id+"/submit", ""), 200)

	resp := tc.GetBearer(t, "/orders/queries/pending", token)
	tc.AssertStatus(t, resp, 200)
	tc.AssertBodyContains(t, resp, id)
}
