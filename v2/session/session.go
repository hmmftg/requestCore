// Package session provides pluggable session and flash management for v2.
//
// The Store interface abstracts session persistence. The default CookieStore
// stores signed session data in an HTTP cookie. Alternative implementations
// (Redis, database) can use the opaque token as a session ID.
//
// Flash provides one-shot messages that survive a single redirect.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Session represents a user session with key-value storage.
type Session struct {
	id        string
	data      map[string]any
	store     Store
	mu        sync.RWMutex
	dirty     bool
	createdAt time.Time

	// revision is incremented on every mutation (Set/Delete/Clear/
	// flash persistence). Save uses it to detect concurrent changes:
	// if the revision has advanced since the snapshot was taken, the
	// dirty flag is not cleared.
	revision uint64
}

// ID returns the opaque session identifier.
func (s *Session) ID() string {
	return s.id
}

// Get retrieves a value by key. Returns nil if the key does not exist.
func (s *Session) Get(key string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

// GetString retrieves a string value by key. Returns "" if the key does not
// exist or the value is not a string.
func (s *Session) GetString(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.data[key].(string); ok {
		return v
	}
	return ""
}

// GetInto unmarshals a JSON-encoded session value into the target.
// Returns an error if the key does not exist or unmarshaling fails.
func (s *Session) GetInto(key string, target any) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	if !ok {
		return fmt.Errorf("session: key %q not found", key)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("session: marshal key %q: %w", key, err)
	}
	return json.Unmarshal(raw, target)
}

// Set stores a value by key and marks the session as dirty.
func (s *Session) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]any)
	}
	s.data[key] = value
	s.dirty = true
	s.revision++
}

// Delete removes a key from the session and marks it dirty.
func (s *Session) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	s.dirty = true
	s.revision++
}

// Clear removes all data from the session and marks it dirty.
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]any)
	s.dirty = true
	s.revision++
}

// IsDirty reports whether the session has unsaved changes.
func (s *Session) IsDirty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dirty
}

// CreatedAt returns the session creation time.
func (s *Session) CreatedAt() time.Time {
	return s.createdAt
}

// Save persists the session through its store and returns the opaque token.
// It takes a snapshot of the session data under the read lock, then saves
// the snapshot outside the lock. After saving, it reacquires the lock and
// clears the dirty flag only if the revision has not advanced (i.e., no
// concurrent mutations occurred during the save).
func (s *Session) Save(ctx context.Context) (string, error) {
	// Snapshot under the read lock.
	s.mu.RLock()
	snapshot := &Session{
		id:        s.id,
		data:      copyMap(s.data),
		store:     s.store,
		createdAt: s.createdAt,
		revision:  s.revision,
	}
	s.mu.RUnlock()

	// Save the detached snapshot outside the lock.
	token, err := s.store.Save(ctx, snapshot)
	if err != nil {
		return "", fmt.Errorf("session: save: %w", err)
	}

	// Reacquire the write lock and clear dirty only if no concurrent
	// mutations occurred during the save.
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.revision == snapshot.revision {
		s.dirty = false
	}
	return token, nil
}

// copyMap creates a shallow copy of a map[string]any.
func copyMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// Destroy deletes the session from its store.
func (s *Session) Destroy(ctx context.Context) error {
	return s.store.Delete(ctx, s.id)
}

// Data returns a copy of the session data map.
func (s *Session) Data() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]any, len(s.data))
	for k, v := range s.data {
		result[k] = v
	}
	return result
}

// Store is the persistence backend for sessions.
// The token is an opaque string that the framework adapter stores in a cookie.
// For CookieStore, the token contains the signed payload.
// For server-side stores, the token is a session ID.
type Store interface {
	// Load retrieves a session by its opaque token.
	// Returns an error if the token is invalid, expired, or not found.
	Load(ctx context.Context, token string) (*Session, error)

	// Save persists a session and returns its opaque token.
	Save(ctx context.Context, s *Session) (string, error)

	// Delete removes a session by its opaque token.
	Delete(ctx context.Context, token string) error
}

// NewSession creates a new empty session backed by the given store.
func NewSession(store Store) *Session {
	return &Session{
		id:        generateID(),
		data:      make(map[string]any),
		store:     store,
		createdAt: time.Now(),
	}
}
