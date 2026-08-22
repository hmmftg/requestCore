package session

import (
	"context"
	"sync"
)

// Flash provides one-shot messages that survive a single redirect.
// Reading a flash value consumes it for the next save cycle.
type Flash struct {
	mu      sync.Mutex
	entries map[string]string
	read    map[string]bool
}

// NewFlash creates a new empty Flash.
func NewFlash() *Flash {
	return &Flash{
		entries: make(map[string]string),
		read:    make(map[string]bool),
	}
}

// Add stores a flash message by key.
func (f *Flash) Add(key, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries[key] = value
	delete(f.read, key)
}

// Get retrieves a flash message by key and marks it as read (consumed).
// Returns "" if the key does not exist or has already been consumed.
func (f *Flash) Get(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.read[key] {
		return ""
	}
	v, ok := f.entries[key]
	if !ok {
		return ""
	}
	f.read[key] = true
	return v
}

// Peek retrieves a flash message by key without marking it as read.
// Returns "" if the key does not exist or has already been consumed.
func (f *Flash) Peek(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.read[key] {
		return ""
	}
	return f.entries[key]
}

// GetAll returns all unconsumed flash entries and marks them as read (consumed).
func (f *Flash) GetAll() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make(map[string]string, len(f.entries))
	for k, v := range f.entries {
		if f.read[k] {
			continue
		}
		result[k] = v
		f.read[k] = true
	}
	return result
}

// Has reports whether a flash key exists and has not been consumed.
func (f *Flash) Has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.read[key] {
		return false
	}
	_, ok := f.entries[key]
	return ok
}

// Clear marks all flash entries as consumed.
func (f *Flash) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k := range f.entries {
		f.read[k] = true
	}
}

// ConsumedEntries returns the keys that have been read and should be
// removed on the next save cycle.
func (f *Flash) ConsumedEntries() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var keys []string
	for k := range f.read {
		if f.read[k] {
			keys = append(keys, k)
		}
	}
	return keys
}

// ActiveEntries returns the flash entries that have not been consumed.
func (f *Flash) ActiveEntries() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make(map[string]string)
	for k, v := range f.entries {
		if !f.read[k] {
			result[k] = v
		}
	}
	return result
}

// LoadFromSession restores flash entries from a session's reserved namespace.
const flashKey = "_flash"

// LoadFlashFromSession creates a Flash populated from session data.
// It handles both map[string]string (direct) and map[string]any
// (after JSON round-trip through CookieStore) flash data.
func LoadFlashFromSession(s *Session) *Flash {
	f := NewFlash()
	if s == nil {
		return f
	}
	if raw := s.Get(flashKey); raw != nil {
		switch entries := raw.(type) {
		case map[string]string:
			for k, v := range entries {
				f.entries[k] = v
			}
		case map[string]any:
			// After JSON round-trip through CookieStore, map values
			// are unmarshaled as any. Convert string values back.
			for k, v := range entries {
				if sv, ok := v.(string); ok {
					f.entries[k] = sv
				}
			}
		}
	}
	return f
}

// SaveFlashToSession stores active (unconsumed) flash entries into a session.
func SaveFlashToSession(s *Session, f *Flash) {
	if s == nil || f == nil {
		return
	}
	active := f.ActiveEntries()
	if len(active) == 0 {
		s.Delete(flashKey)
		return
	}
	s.Set(flashKey, active)
}

// NoOpStore is a Store implementation that does nothing.
// It is useful for testing and for applications that do not need sessions.
type NoOpStore struct{}

// Load always returns a new empty session.
func (NoOpStore) Load(_ context.Context, _ string) (*Session, error) {
	return NewSession(NoOpStore{}), nil
}

// Save returns the session ID unchanged.
func (NoOpStore) Save(_ context.Context, s *Session) (string, error) {
	return s.id, nil
}

// Delete is a no-op.
func (NoOpStore) Delete(_ context.Context, _ string) error {
	return nil
}
