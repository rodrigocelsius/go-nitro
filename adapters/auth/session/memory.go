package session

import (
	"context"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*domain.Session
	creates  int // Create calls since the last expired-session sweep
}

// memorySweepEvery bounds memory: expired sessions that are never read again
// (the user just left) are purged every memorySweepEvery creates. Without it
// every login leaked one entry for the life of the process.
const memorySweepEvery = 1024

// NewMemory creates a new in-memory session store.
func NewMemory() domain.SessionStore {
	return &MemoryStore{sessions: make(map[string]*domain.Session)}
}

func (s *MemoryStore) Create(ctx context.Context, sess *domain.Session) error {
	if sess.ID == "" {
		sess.ID = NewID()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.creates++; s.creates >= memorySweepEvery {
		s.creates = 0
		now := time.Now()
		for id, old := range s.sessions {
			if now.After(old.ExpiresAt) {
				delete(s.sessions, id)
			}
		}
	}
	s.sessions[sess.ID] = sess
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, id string) (*domain.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	if !ok || time.Now().After(sess.ExpiresAt) {
		return nil, domain.ErrNotFound
	}
	return sess, nil
}

func (s *MemoryStore) Update(ctx context.Context, sess *domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[sess.ID]; !ok {
		return domain.ErrNotFound
	}
	s.sessions[sess.ID] = sess
	return nil
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}

func (s *MemoryStore) DeleteByUserID(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.sessions {
		if sess.UserID == userID {
			delete(s.sessions, id)
		}
	}
	return nil
}
