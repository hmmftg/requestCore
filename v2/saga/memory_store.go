package saga

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of both SagaStore and
// OutboxStore. It is safe for concurrent use and suitable for tests
// and single-process services.
type MemoryStore struct {
	mu     sync.Mutex
	sagas  map[string]*SagaState
	claims map[string]string
	outbox map[string]*OutboxRecord
	clock  func() time.Time
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sagas:  make(map[string]*SagaState),
		claims: make(map[string]string),
		outbox: make(map[string]*OutboxRecord),
		clock:  time.Now,
	}
}

// Save creates or replaces a saga state record.
func (m *MemoryStore) Save(_ context.Context, st *SagaState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sagas[st.ID] = deepCopySagaState(st)
	return nil
}

// Load retrieves a saga state by ID.
func (m *MemoryStore) Load(_ context.Context, sagaID string) (*SagaState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.sagas[sagaID]
	if !ok {
		return nil, fmt.Errorf("saga: not found: %s", sagaID)
	}
	return deepCopySagaState(st), nil
}

// UpdateStepAndOutbox atomically updates a step's state and appends
// outbox records.
func (m *MemoryStore) UpdateStepAndOutbox(_ context.Context, sagaID string, stepIdx int, step StepState, outbox []OutboxRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.sagas[sagaID]
	if !ok {
		return fmt.Errorf("saga: not found: %s", sagaID)
	}
	if stepIdx < 0 || stepIdx >= len(st.Steps) {
		return fmt.Errorf("saga: step index out of range: %d", stepIdx)
	}
	st.Steps[stepIdx] = step
	st.UpdatedAt = m.clock()
	for i := range outbox {
		rec := outbox[i]
		if rec.ID == "" {
			return fmt.Errorf("saga: outbox record ID cannot be empty")
		}
		if rec.Status == "" {
			rec.Status = OutboxPending
		}
		recCopy := rec
		m.outbox[rec.ID] = &recCopy
	}
	return nil
}

// ListIncomplete returns all sagas with status Running or Compensating.
func (m *MemoryStore) ListIncomplete(_ context.Context) ([]*SagaState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*SagaState
	for _, st := range m.sagas {
		if st.Status == SagaRunning || st.Status == SagaCompensating {
			result = append(result, deepCopySagaState(st))
		}
	}
	return result, nil
}

// ClaimSaga atomically marks a saga as being resumed by this instance.
func (m *MemoryStore) ClaimSaga(_ context.Context, sagaID string, claimedBy string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.claims[sagaID]
	if ok && existing != claimedBy {
		return false, nil
	}
	m.claims[sagaID] = claimedBy
	return true, nil
}

// ClearClaim releases the claim on a saga.
func (m *MemoryStore) ClearClaim(_ context.Context, sagaID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.claims, sagaID)
	return nil
}

// AppendOutbox adds a record to the outbox.
func (m *MemoryStore) AppendOutbox(_ context.Context, rec OutboxRecord) error {
	if rec.ID == "" {
		return fmt.Errorf("saga: outbox record ID cannot be empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	recCopy := rec
	m.outbox[rec.ID] = &recCopy
	return nil
}

// ListPendingOutbox returns up to limit pending outbox records.
func (m *MemoryStore) ListPendingOutbox(_ context.Context, limit int) ([]OutboxRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []OutboxRecord
	for _, rec := range m.outbox {
		if rec.Status == OutboxPending {
			result = append(result, *rec)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// MarkPublished marks a record as successfully published.
func (m *MemoryStore) MarkPublished(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.outbox[id]
	if !ok {
		return fmt.Errorf("saga: outbox record not found: %s", id)
	}
	now := m.clock()
	rec.Status = OutboxPublished
	rec.PublishedAt = &now
	rec.LastError = ""
	return nil
}

// MarkFailed increments the fail count and, if the threshold is
// reached, marks the record as Failed.
func (m *MemoryStore) MarkFailed(_ context.Context, id string, errMsg string, threshold int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.outbox[id]
	if !ok {
		return fmt.Errorf("saga: outbox record not found: %s", id)
	}
	rec.FailCount++
	rec.LastError = errMsg
	if threshold > 0 && rec.FailCount >= threshold {
		rec.Status = OutboxFailed
	}
	return nil
}
