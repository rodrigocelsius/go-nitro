package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Expired sessions that are never read again used to stay in memory forever.
func TestMemoryStore_SweepsExpiredSessions(t *testing.T) {
	s := NewMemory().(*MemoryStore)
	ctx := context.Background()
	_ = s.Create(ctx, &domain.Session{ID: "gone", UserID: "u", ExpiresAt: time.Now().Add(-time.Minute)})
	for i := 0; i < memorySweepEvery; i++ {
		_ = s.Create(ctx, &domain.Session{ID: fmt.Sprint(i), UserID: "u", ExpiresAt: time.Now().Add(time.Hour)})
	}
	s.mu.RLock()
	_, still := s.sessions["gone"]
	s.mu.RUnlock()
	if still {
		t.Fatal("expired session must be swept")
	}
}
