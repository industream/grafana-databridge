package cache

import (
	"sync"
	"time"
)

// Entry holds a cached value with an expiration time.
type Entry[T any] struct {
	Data      T
	ExpiresAt time.Time
}

// IsExpired returns true if the entry has expired.
func (e *Entry[T]) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// Store is a generic, thread-safe, TTL-based in-memory cache.
type Store[T any] struct {
	mu    sync.RWMutex
	items map[string]Entry[T]
	ttl   time.Duration
}

// NewStore creates a cache store with the given TTL.
func NewStore[T any](ttl time.Duration) *Store[T] {
	return &Store[T]{
		items: make(map[string]Entry[T]),
		ttl:   ttl,
	}
}

// Get returns the cached value for the key, or false if not found/expired.
func (s *Store[T]) Get(key string) (T, bool) {
	s.mu.RLock()
	entry, ok := s.items[key]
	s.mu.RUnlock()

	if !ok || entry.IsExpired() {
		var zero T
		return zero, false
	}
	return entry.Data, true
}

// Set stores a value with the configured TTL.
func (s *Store[T]) Set(key string, value T) {
	s.mu.Lock()
	s.items[key] = Entry[T]{
		Data:      value,
		ExpiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
}

// Invalidate removes a specific key from the cache.
func (s *Store[T]) Invalidate(key string) {
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
}

// Clear removes all entries from the cache.
func (s *Store[T]) Clear() {
	s.mu.Lock()
	s.items = make(map[string]Entry[T])
	s.mu.Unlock()
}
