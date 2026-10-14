package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// RedisStore implements domain.SessionStore using Redis.
type RedisStore struct {
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
}

// NewRedis creates a new Redis-backed session store.
// The ttl parameter controls the maximum lifetime of sessions in Redis.
// Session expiration is handled by Redis key expiration (TTL).
func NewRedis(client *redis.Client, ttl time.Duration) domain.SessionStore {
	return &RedisStore{
		client:    client,
		keyPrefix: "nitro:session:",
		ttl:       ttl,
	}
}

// sessionData is the internal representation stored in Redis.
// We store a flat structure to make serialization easier.
type sessionData struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Data      map[string]any `json:"data,omitempty"`
	ExpiresAt time.Time      `json:"expires_at"`
	CreatedAt time.Time      `json:"created_at"`
}

func (s *RedisStore) key(id string) string {
	return s.keyPrefix + id
}

// userSessionsKey returns the key for the user-to-session-IDs index.
func (s *RedisStore) userSessionsKey(userID string) string {
	return s.keyPrefix + "user:" + userID
}

func (s *RedisStore) Create(ctx context.Context, sess *domain.Session) error {
	if sess.ID == "" {
		sess.ID = NewID()
	}

	data := sessionData{
		ID:        sess.ID,
		UserID:    sess.UserID,
		Data:      sess.Data,
		ExpiresAt: sess.ExpiresAt,
		CreatedAt: sess.CreatedAt,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("session: marshal: %w", err)
	}

	// Calculate TTL from session expiration
	ttl := time.Until(sess.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("session: session is already expired")
	}

	// Use a pipeline to execute both commands atomically
	pipe := s.client.Pipeline()
	pipe.Set(ctx, s.key(sess.ID), jsonBytes, ttl)
	// Index: add session ID to the user's set of sessions
	pipe.SAdd(ctx, s.userSessionsKey(sess.UserID), sess.ID)
	// Set TTL on the user index key (use max of session TTL and a reasonable default)
	pipe.Expire(ctx, s.userSessionsKey(sess.UserID), max(ttl, 24*time.Hour))

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("session: pipeline exec: %w", err)
	}

	return nil
}

func (s *RedisStore) Get(ctx context.Context, id string) (*domain.Session, error) {
	jsonBytes, err := s.client.Get(ctx, s.key(id)).Bytes()
	if err == redis.Nil {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("session: get: %w", err)
	}

	var data sessionData
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return nil, fmt.Errorf("session: unmarshal: %w", err)
	}

	// Check expiration (belt and suspenders — Redis TTL should handle this)
	if time.Now().After(data.ExpiresAt) {
		// Clean up expired session
		_ = s.Delete(ctx, id)
		return nil, domain.ErrNotFound
	}

	return &domain.Session{
		ID:        data.ID,
		UserID:    data.UserID,
		Data:      data.Data,
		ExpiresAt: data.ExpiresAt,
		CreatedAt: data.CreatedAt,
	}, nil
}

func (s *RedisStore) Update(ctx context.Context, sess *domain.Session) error {
	// Check if session exists first
	exists, err := s.client.Exists(ctx, s.key(sess.ID)).Result()
	if err != nil {
		return fmt.Errorf("session: exists check: %w", err)
	}
	if exists == 0 {
		return domain.ErrNotFound
	}

	data := sessionData{
		ID:        sess.ID,
		UserID:    sess.UserID,
		Data:      sess.Data,
		ExpiresAt: sess.ExpiresAt,
		CreatedAt: sess.CreatedAt,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("session: marshal: %w", err)
	}

	// Calculate remaining TTL
	ttl := time.Until(sess.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("session: session is already expired")
	}

	err = s.client.Set(ctx, s.key(sess.ID), jsonBytes, ttl).Err()
	if err != nil {
		return fmt.Errorf("session: update: %w", err)
	}

	return nil
}

func (s *RedisStore) Delete(ctx context.Context, id string) error {
	// Get session data before deletion to know which user index to update
	jsonBytes, err := s.client.Get(ctx, s.key(id)).Bytes()
	if err == redis.Nil {
		// Session doesn't exist, nothing to do
		return nil
	}
	if err != nil {
		return fmt.Errorf("session: get for delete: %w", err)
	}

	var data sessionData
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return fmt.Errorf("session: unmarshal for delete: %w", err)
	}

	pipe := s.client.Pipeline()
	pipe.Del(ctx, s.key(id))
	pipe.SRem(ctx, s.userSessionsKey(data.UserID), id)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("session: pipeline exec delete: %w", err)
	}

	return nil
}

func (s *RedisStore) DeleteByUserID(ctx context.Context, userID string) error {
	// Get all session IDs for this user
	sessionIDs, err := s.client.SMembers(ctx, s.userSessionsKey(userID)).Result()
	if err != nil {
		return fmt.Errorf("session: smembers: %w", err)
	}

	if len(sessionIDs) == 0 {
		return nil
	}

	// Delete all sessions and the user index
	pipe := s.client.Pipeline()
	for _, sid := range sessionIDs {
		pipe.Del(ctx, s.key(sid))
	}
	pipe.Del(ctx, s.userSessionsKey(userID))

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("session: pipeline exec delete by user: %w", err)
	}

	return nil
}

// max returns the larger of two durations.
func max(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
