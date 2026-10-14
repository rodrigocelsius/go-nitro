package nitro

import "sync"

// SafeMap is a generic thread-safe map.
// It removes the need for a manual sync.RWMutex in repositories.
//
// Example:
//
//	type InMemoryTodoRepo struct {
//	    items nitro.SafeMap[string, *Todo]
//	}
//
//	func (r *InMemoryTodoRepo) FindByID(_ context.Context, id any) (any, error) {
//	    t, ok := r.items.Get(fmt.Sprint(id))
//	    if !ok {
//	        return nil, nitro.NotFound("todo not found")
//	    }
//	    return t, nil
//	}
type SafeMap[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

// NewSafeMap creates an empty SafeMap.
func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{items: make(map[K]V)}
}

// Get returns the value for the key and whether it exists.
func (m *SafeMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.items[key]
	return v, ok
}

// Set stores the value for the key, overwriting any existing value.
func (m *SafeMap[K, V]) Set(key K, val V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = val
}

// SetIfAbsent stores the value only when the key is not present and reports
// whether it was stored (atomic check-and-set).
func (m *SafeMap[K, V]) SetIfAbsent(key K, val V) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.items[key]; ok {
		return false
	}
	m.items[key] = val
	return true
}

// Delete removes the key from the map.
func (m *SafeMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
}

// Has reports whether the key exists in the map.
func (m *SafeMap[K, V]) Has(key K) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.items[key]
	return ok
}

// Values returns a slice of all map values.
func (m *SafeMap[K, V]) Values() []V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]V, 0, len(m.items))
	for _, v := range m.items {
		out = append(out, v)
	}
	return out
}

// Keys returns a slice of all map keys.
func (m *SafeMap[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]K, 0, len(m.items))
	for k := range m.items {
		out = append(out, k)
	}
	return out
}

// Len returns the number of elements in the map.
func (m *SafeMap[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// Range walks the map calling fn for each key-value pair.
// If fn returns false, iteration stops.
//
// fn runs WITHOUT the internal lock held, so it may safely call
// Set/Delete/Get/Clear on the same map. Entries added or removed during
// iteration may or may not be visited.
func (m *SafeMap[K, V]) Range(fn func(key K, value V) bool) {
	for _, k := range m.Keys() {
		v, ok := m.Get(k)
		if !ok {
			continue // removed while iterating
		}
		if !fn(k, v) {
			return
		}
	}
}

// Clear removes all elements from the map.
func (m *SafeMap[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.items)
}
