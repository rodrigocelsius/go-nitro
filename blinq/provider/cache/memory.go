package cache

import (
	"context"
	"sync"
	"time"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/provider"
)

var _ provider.Cache = (*Memory)(nil)

// Entry is a cached value with expiration and an optional tag.
type entry struct {
	rows      []map[string]any
	expiresAt time.Time
	tag       string
}

// Memory is a thread-safe in-process cache (pluggable via provider.Cache).
type Memory struct {
	mu    sync.RWMutex
	items map[string]entry
	byTag map[string]map[string]struct{}
	now   func() time.Time
}

// New creates an in-memory cache.
func New() *Memory {
	return &Memory{
		items: make(map[string]entry),
		byTag: make(map[string]map[string]struct{}),
		now:   time.Now,
	}
}

func (m *Memory) Get(_ context.Context, key string) ([]map[string]any, bool) {
	m.mu.RLock()
	e, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !e.expiresAt.IsZero() && m.now().After(e.expiresAt) {
		m.mu.Lock()
		delete(m.items, key)
		m.mu.Unlock()
		return nil, false
	}
	out := make([]map[string]any, len(e.rows))
	for i, r := range e.rows {
		cp := make(map[string]any, len(r))
		for k, v := range r {
			cp[k] = v
		}
		out[i] = cp
	}
	return out, true
}

func (m *Memory) Set(_ context.Context, key string, rows []map[string]any, ttl time.Duration) {
	cp := make([]map[string]any, len(rows))
	for i, r := range rows {
		row := make(map[string]any, len(r))
		for k, v := range r {
			row[k] = v
		}
		cp[i] = row
	}
	tag := ""
	if i := indexTag(key); i >= 0 {
		tag = key[:i]
	}
	e := entry{rows: cp, tag: tag}
	if ttl > 0 {
		e.expiresAt = m.now().Add(ttl)
	}
	m.mu.Lock()
	m.items[key] = e
	if tag != "" {
		if m.byTag[tag] == nil {
			m.byTag[tag] = make(map[string]struct{})
		}
		m.byTag[tag][key] = struct{}{}
	}
	m.mu.Unlock()
}

func (m *Memory) InvalidateTag(_ context.Context, tag string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := m.byTag[tag]
	for k := range keys {
		delete(m.items, k)
	}
	delete(m.byTag, tag)
}

func indexTag(key string) int {
	for i := 0; i < len(key); i++ {
		if key[i] == '|' {
			return i
		}
	}
	return -1
}
