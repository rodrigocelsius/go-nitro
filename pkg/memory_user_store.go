package nitro

import (
	"context"
	"strings"
	"time"
)

// MemoryUserStore is an in-memory UserLookup + UserCreator for development/tests.
type MemoryUserStore struct {
	users   *SafeMap[string, *User]
	byEmail *SafeMap[string, *User]
}

// NewMemoryUserStore creates an empty in-memory user store.
func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{
		users:   NewSafeMap[string, *User](),
		byEmail: NewSafeMap[string, *User](),
	}
}

func (s *MemoryUserStore) FindByEmail(_ context.Context, email string) (*User, error) {
	u, ok := s.byEmail.Get(email)
	if !ok {
		return nil, NotFound("user not found")
	}
	return u, nil
}

func (s *MemoryUserStore) FindByID(_ context.Context, id string) (*User, error) {
	u, ok := s.users.Get(id)
	if !ok {
		return nil, NotFound("user not found")
	}
	return u, nil
}

func (s *MemoryUserStore) Create(_ context.Context, user *User) error {
	id := NextIDString()
	user.ID = strings.TrimSpace(user.ID)
	if user.ID == "" {
		user.ID = id
	}
	if user.Roles == nil {
		user.Roles = []string{"user"}
	}
	user.Active = true
	now := time.Now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now
	s.users.Set(user.ID, user)
	s.byEmail.Set(user.Email, user)
	return nil
}

// Len returns how many users are stored.
func (s *MemoryUserStore) Len() int {
	return s.users.Len()
}
