package idempotency

import (
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of Store, suitable for
// testing and single-process development. It is NOT suitable for
// production use because it does not survive restarts and does not
// work across multiple process instances.
type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]*Record
}

// NewMemoryStore creates a new in-memory idempotency store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records: make(map[string]*Record),
	}
}

// Reserve attempts to create an in-progress record for the given key.
func (s *MemoryStore) Reserve(key string, fingerprint RequestFingerprint, ttl time.Duration) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	if existing, ok := s.records[key]; ok {
		if existing.IsExpired(now) {
			delete(s.records, key)
		} else if existing.Fingerprint.Hash() != fingerprint.Hash() {
			return existing, ErrFingerprintMismatch
		} else if existing.State == StateInProgress {
			return existing, ErrConflict
		} else if existing.State == StateCompleted {
			return existing, ErrReplayAvailable
		}
		// StateFailed: allow re-reservation (fall through)
	}

	record := &Record{
		Key:         key,
		Fingerprint: fingerprint,
		State:       StateInProgress,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
	}
	s.records[key] = record
	return nil, nil
}

// Complete marks an in-progress record as completed.
func (s *MemoryStore) Complete(key string, status int, headers map[string][]string, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.records[key]
	if !ok {
		return ErrNotFound
	}
	if record.State != StateInProgress {
		return ErrNotInProgress
	}

	record.State = StateCompleted
	record.ResponseStatus = status
	record.ResponseHeaders = headers
	record.ResponseBody = append([]byte(nil), body...)
	return nil
}

// Fail marks an in-progress record as failed.
func (s *MemoryStore) Fail(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.records[key]
	if !ok {
		return ErrNotFound
	}
	if record.State != StateInProgress {
		return ErrNotInProgress
	}

	record.State = StateFailed
	return nil
}

// Get retrieves the record for the given key.
func (s *MemoryStore) Get(key string) (*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[key]
	if !ok {
		return nil, nil
	}
	if record.IsExpired(time.Now()) {
		return nil, nil
	}
	return record, nil
}

// Cleanup removes expired records. This is a maintenance operation
// that should be called periodically.
func (s *MemoryStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, record := range s.records {
		if record.IsExpired(now) {
			delete(s.records, key)
		}
	}
}

// Len returns the number of records in the store.
func (s *MemoryStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// Additional sentinel errors for Store operations.
var (
	// ErrNotFound indicates the record was not found.
	ErrNotFound = errNotFound{}

	// ErrNotInProgress indicates the record is not in-progress.
	ErrNotInProgress = errNotInProgress{}
)

type errNotFound struct{}

func (errNotFound) Error() string { return "idempotency: record not found" }

type errNotInProgress struct{}

func (errNotInProgress) Error() string { return "idempotency: record not in-progress" }
