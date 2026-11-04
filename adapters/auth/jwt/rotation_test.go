package jwt

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Concurrent refreshes with the same refresh token used to race between the
// revocation check and the revoke, minting several valid pairs from one token.
func TestProvider_RefreshToken_RedeemableOnceUnderConcurrency(t *testing.T) {
	p := MustNew(testCfg())
	ctx := context.Background()
	refresh, err := p.GenerateRefreshToken(ctx, &domain.User{ID: "u1"})
	if err != nil {
		t.Fatal(err)
	}

	var ok atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := p.RefreshToken(ctx, refresh); err == nil {
				ok.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := ok.Load(); got != 1 {
		t.Fatalf("refresh token redeemed %d times, want exactly 1", got)
	}
}

// failingRevokeStore implements only RevocationStore (no Consume), so the
// provider takes the check-then-revoke fallback.
type failingRevokeStore struct{}

func (f *failingRevokeStore) Revoked(context.Context, string) (bool, error) { return false, nil }
func (f *failingRevokeStore) Revoke(context.Context, string, time.Time) error {
	return errors.New("store down")
}

// A store that cannot record the rotation must not leave the old refresh
// token usable (the error used to be ignored).
func TestProvider_RefreshToken_FailsClosedWhenRevokeFails(t *testing.T) {
	p := MustNew(testCfg())
	p.SetRevocationStore(&failingRevokeStore{})
	ctx := context.Background()
	refresh, _ := p.GenerateRefreshToken(ctx, &domain.User{ID: "u1"})
	if _, err := p.RefreshToken(ctx, refresh); err == nil {
		t.Fatal("refresh must fail when the rotation cannot be recorded")
	}
}

func TestMemoryStore_ConsumeOnce(t *testing.T) {
	m := newMemoryStore()
	ctx := context.Background()
	exp := time.Now().Add(time.Hour)
	if first, _ := m.Consume(ctx, "j", exp); !first {
		t.Fatal("first consume must succeed")
	}
	if first, _ := m.Consume(ctx, "j", exp); first {
		t.Fatal("second consume must fail")
	}
	if revoked, _ := m.Revoked(ctx, "j"); !revoked {
		t.Fatal("consumed jti must read as revoked")
	}
}

func TestMemoryStore_SweepIsAmortized(t *testing.T) {
	m := newMemoryStore()
	ctx := context.Background()
	_ = m.Revoke(ctx, "old", time.Now().Add(-time.Second))
	_ = m.Revoke(ctx, "new", time.Now().Add(time.Hour))
	if len(m.revoked) != 2 {
		t.Fatalf("no sweep expected within the interval, got %d entries", len(m.revoked))
	}
	m.lastSweep = time.Now().Add(-2 * memorySweepInterval)
	_ = m.Revoke(ctx, "newer", time.Now().Add(time.Hour))
	if _, ok := m.revoked["old"]; ok {
		t.Fatal("expired jti must be swept once the interval elapsed")
	}
}
