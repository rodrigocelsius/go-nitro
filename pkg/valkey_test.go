package nitro

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/resilience/v2"
)

func clearValkeyEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"VALKEY_ADDR", "REDIS_ADDR", "VALKEY_PASSWORD", "REDIS_PASSWORD", "VALKEY_DB", "REDIS_DB", "VALKEY_TLS", "REDIS_TLS"} {
		t.Setenv(k, "")
	}
}

func TestValkeyConfigFallsBackToRedisEnv(t *testing.T) {
	clearValkeyEnv(t)
	t.Setenv("REDIS_ADDR", "cache:6379")
	t.Setenv("REDIS_DB", "2")
	cfg, err := ValkeyConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "cache:6379" || cfg.DB != 2 {
		t.Fatalf("cfg = %+v, want REDIS_* used as fallback", cfg)
	}

	t.Setenv("VALKEY_ADDR", "valkey:6379")
	cfg, err = ValkeyConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "valkey:6379" {
		t.Fatalf("Addr = %q, want VALKEY_ADDR to win", cfg.Addr)
	}
}

func TestValkeyClientFromEnvIsNilWhenUnset(t *testing.T) {
	clearValkeyEnv(t)
	rdb, err := ValkeyClientFromEnv()
	if err != nil || rdb != nil {
		t.Fatalf("rdb=%v err=%v, want nil, nil", rdb, err)
	}
}

func TestValkeyTLSUsesTheHostAsServerName(t *testing.T) {
	rdb := NewValkeyClient(ValkeyConfig{Addr: "cache.internal:6380", TLS: true})
	defer rdb.Close()
	tc := rdb.Options().TLSConfig
	if tc == nil || tc.ServerName != "cache.internal" || tc.MinVersion == 0 {
		t.Fatalf("TLS config = %+v", tc)
	}
}

// Runs the Redis adapters against a real Valkey server:
//
//	docker run -d --rm -p 127.0.0.1:26379:6379 valkey/valkey:8-alpine
//	NITRO_TEST_VALKEY_ADDR=127.0.0.1:26379 go test -run Valkey ./...
func TestLiveValkeyAdapters(t *testing.T) {
	addr := strings.TrimSpace(os.Getenv("NITRO_TEST_VALKEY_ADDR"))
	if addr == "" {
		t.Skip("set NITRO_TEST_VALKEY_ADDR to run against a Valkey server")
	}
	ctx := context.Background()
	rdb := NewValkeyClient(ValkeyConfig{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })

	info, err := rdb.Info(ctx, "server").Result()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info, "valkey_version") {
		t.Fatalf("not a Valkey server:\n%s", info)
	}

	prefix := "nitro-test:" + time.Now().Format("150405.000000") + ":"
	cache := ValkeyCache(rdb, WithRedisKeyPrefix(prefix))
	type row struct {
		Name string `json:"name"`
	}
	if err := cache.Set("k", row{Name: "Ana"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	var got row
	if err := cache.Get("k", &got); err != nil || got.Name != "Ana" {
		t.Fatalf("cache get = %+v, %v", got, err)
	}
	if err := cache.Delete("k"); err != nil {
		t.Fatal(err)
	}
	if err := cache.Get("k", &got); err == nil {
		t.Fatal("expected a miss after Delete")
	}

	sessions := ValkeySessionStore(rdb, time.Minute)
	s := &domain.Session{ID: prefix + "s1", UserID: prefix + "u1", Data: map[string]any{"role": "admin"}, ExpiresAt: time.Now().Add(time.Minute)}
	if err := sessions.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	back, err := sessions.Get(ctx, s.ID)
	if err != nil || back.UserID != s.UserID || back.Data["role"] != "admin" {
		t.Fatalf("session get = %+v, %v", back, err)
	}
	if err := sessions.DeleteByUserID(ctx, s.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Get(ctx, s.ID); err == nil {
		t.Fatal("expected the session to be gone after DeleteByUserID")
	}

	breakers := NewValkeyBreakerStore(rdb, prefix+"cb:", time.Minute)
	at := time.Now().UTC().Truncate(time.Second)
	if err := breakers.Set(ctx, "payments", 3, at, resilience.CircuitOpen); err != nil {
		t.Fatal(err)
	}
	failures, last, state, ok, err := breakers.Get(ctx, "payments")
	if err != nil || !ok || failures != 3 || state != resilience.CircuitOpen || !last.Equal(at) {
		t.Fatalf("breaker get = %d %v %v %v %v", failures, last, state, ok, err)
	}
}
