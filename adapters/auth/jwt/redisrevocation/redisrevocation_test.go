package redisrevocation

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// TestStore_RevokeAndRevoked covers V-26: a Redis-backed
// RevocationStore lets jwt.Provider share revocations across replicas,
// unlike the in-process default.
func TestStore_RevokeAndRevoked(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	store := New(client)
	ctx := context.Background()

	revoked, err := store.Revoked(ctx, "jti-1")
	if err != nil {
		t.Fatal(err)
	}
	if revoked {
		t.Fatal("expected jti-1 to not be revoked yet")
	}

	if err := store.Revoke(ctx, "jti-1", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	revoked, err = store.Revoked(ctx, "jti-1")
	if err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("expected jti-1 to be revoked")
	}

	// Empty jti is a no-op both ways.
	if err := store.Revoke(ctx, "", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if revoked, err := store.Revoked(ctx, ""); err != nil || revoked {
		t.Fatalf("expected empty jti to be unrevoked, got revoked=%v err=%v", revoked, err)
	}
}

// TestStore_RevokeAlreadyExpired ensures revoking a token whose exp is
// already in the past is a harmless no-op instead of an error.
func TestStore_RevokeAlreadyExpired(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	store := New(client)
	ctx := context.Background()

	if err := store.Revoke(ctx, "jti-expired", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	revoked, err := store.Revoked(ctx, "jti-expired")
	if err != nil {
		t.Fatal(err)
	}
	if revoked {
		t.Fatal("expected already-expired revoke to be a no-op")
	}
}
