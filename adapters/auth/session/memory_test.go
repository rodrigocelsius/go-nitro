package session

import (
	"context"
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestMemoryStore_CreateAndGet(t *testing.T) {
	store := NewMemory()
	sess := &domain.Session{
		ID:        "session-1",
		UserID:    "user-1",
		Data:      map[string]any{"key": "value"},
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	err := store.Create(context.Background(), sess)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	got, err := store.Get(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.ID != sess.ID {
		t.Fatalf("expected ID %q, got %q", sess.ID, got.ID)
	}
	if got.UserID != sess.UserID {
		t.Fatalf("expected UserID %q, got %q", sess.UserID, got.UserID)
	}
}

func TestMemoryStore_GetExpired(t *testing.T) {
	store := NewMemory()
	sess := &domain.Session{
		ID:        "session-2",
		UserID:    "user-2",
		ExpiresAt: time.Now().Add(-time.Hour), // expired
	}

	_ = store.Create(context.Background(), sess)

	_, err := store.Get(context.Background(), "session-2")
	if err == nil {
		t.Fatal("expected error for expired session")
	}
}

func TestMemoryStore_GetNotFound(t *testing.T) {
	store := NewMemory()
	_, err := store.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemory()
	sess := &domain.Session{
		ID:        "session-3",
		UserID:    "user-3",
		Data:      map[string]any{},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_ = store.Create(context.Background(), sess)

	sess.Data["updated"] = true
	err := store.Update(context.Background(), sess)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	got, _ := store.Get(context.Background(), "session-3")
	if got.Data["updated"] != true {
		t.Fatal("expected updated data")
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemory()
	sess := &domain.Session{
		ID:        "session-4",
		UserID:    "user-4",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_ = store.Create(context.Background(), sess)

	err := store.Delete(context.Background(), "session-4")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	_, err = store.Get(context.Background(), "session-4")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestMemoryStore_DeleteByUserID(t *testing.T) {
	store := NewMemory()
	sess1 := &domain.Session{ID: "s1", UserID: "user-5", ExpiresAt: time.Now().Add(time.Hour)}
	sess2 := &domain.Session{ID: "s2", UserID: "user-5", ExpiresAt: time.Now().Add(time.Hour)}
	sess3 := &domain.Session{ID: "s3", UserID: "user-6", ExpiresAt: time.Now().Add(time.Hour)}

	_ = store.Create(context.Background(), sess1)
	_ = store.Create(context.Background(), sess2)
	_ = store.Create(context.Background(), sess3)

	err := store.DeleteByUserID(context.Background(), "user-5")
	if err != nil {
		t.Fatalf("DeleteByUserID error: %v", err)
	}

	_, err = store.Get(context.Background(), "s1")
	if err == nil {
		t.Fatal("expected error for deleted session s1")
	}
	_, err = store.Get(context.Background(), "s2")
	if err == nil {
		t.Fatal("expected error for deleted session s2")
	}
	// s3 should still exist
	_, err = store.Get(context.Background(), "s3")
	if err != nil {
		t.Fatalf("expected session s3 to exist, got error: %v", err)
	}
}
