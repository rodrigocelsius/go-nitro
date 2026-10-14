package session

import (
	"context"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestNewID_UniqueAndSufficientEntropy(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := NewID()
		if len(id) < 32 {
			t.Fatalf("id too short (%d chars): %q", len(id), id)
		}
		if seen[id] {
			t.Fatalf("duplicate id generated: %q", id)
		}
		seen[id] = true
	}
}

// TestMemoryStore_Create_AutoGeneratesSecureID covers vuln.md V-09: an
// application that forgets to set Session.ID must not end up with an
// empty/predictable identifier.
func TestMemoryStore_Create_AutoGeneratesSecureID(t *testing.T) {
	store := NewMemory()
	sess := &domain.Session{UserID: "user-1", ExpiresAt: time.Now().Add(time.Hour)}

	if err := store.Create(context.Background(), sess); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected Create to fill in a session ID")
	}
	if len(sess.ID) < 32 {
		t.Fatalf("generated ID too short (%d chars): %q", len(sess.ID), sess.ID)
	}

	got, err := store.Get(context.Background(), sess.ID)
	if err != nil || got.UserID != "user-1" {
		t.Fatalf("Get(%q) = %+v, %v", sess.ID, got, err)
	}
}
