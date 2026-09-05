package saga

import (
	"context"
	"encoding/json"
	"time"
)

// SagaStatus represents the overall state of a saga execution.
type SagaStatus string

const (
	// SagaPending means the saga has been created but not started.
	SagaPending SagaStatus = "Pending"

	// SagaRunning means the saga is executing steps forward.
	SagaRunning SagaStatus = "Running"

	// SagaCompleted means all steps completed successfully.
	SagaCompleted SagaStatus = "Completed"

	// SagaCompensating means a step failed and compensation is in progress.
	SagaCompensating SagaStatus = "Compensating"

	// SagaCompensated means all compensations completed.
	SagaCompensated SagaStatus = "Compensated"

	// SagaFailed means the saga failed and compensation also failed.
	SagaFailed SagaStatus = "Failed"
)

// StepStatus represents the state of a single step within a saga.
type StepStatus string

const (
	// StepPending means the step has not been executed yet.
	StepPending StepStatus = "Pending"

	// StepExecuting means the step is currently running.
	StepExecuting StepStatus = "Executing"

	// StepDone means the step completed successfully.
	StepDone StepStatus = "Done"

	// StepCompensating means the step's compensation is in progress.
	StepCompensating StepStatus = "Compensating"

	// StepCompensated means the step's compensation completed.
	StepCompensated StepStatus = "Compensated"

	// StepFailed means the step failed and compensation also failed.
	StepFailed StepStatus = "Failed"
)

// OutboxStatus represents the delivery state of an outbox record.
type OutboxStatus string

const (
	// OutboxPending means the record has not been published yet.
	OutboxPending OutboxStatus = "Pending"

	// OutboxPublished means the record was successfully published.
	OutboxPublished OutboxStatus = "Published"

	// OutboxFailed means the record exceeded the maximum publish attempts.
	OutboxFailed OutboxStatus = "Failed"
)

// SagaState is the durable state of a saga execution. It is persisted
// to a SagaStore for crash recovery and is passed to step functions as
// the scratch pad for inter-step data communication.
type SagaState struct {
	// ID is the unique identifier for this saga execution.
	ID string

	// SagaName is the human-readable name of the saga definition.
	SagaName string

	// Status is the current overall saga status.
	Status SagaStatus

	// Steps tracks the state of each step in execution order.
	Steps []StepState

	// StartedAt is when the saga began executing.
	StartedAt time.Time

	// UpdatedAt is when the saga state was last persisted.
	UpdatedAt time.Time

	// Data is saga-scoped scratch data for step communication.
	// Values are JSON-serializable. Use SetData/GetData helpers for
	// typed access.
	Data map[string]any

	// outboxBuffer accumulates outbox records produced by a step.
	// The orchestrator flushes them atomically with the step state
	// update via SagaStore.UpdateStepAndOutbox.
	outboxBuffer []OutboxRecord
}

// StepState tracks the execution state of a single step within a saga.
type StepState struct {
	// Name identifies the step within the saga.
	Name string

	// Status is the current step status.
	Status StepStatus

	// Attempt is the number of execution attempts for this step.
	Attempt int

	// IdempotencyKey is a unique identifier for this step execution,
	// usable by external systems for deduplication.
	IdempotencyKey IdempotencyKey

	// ExecutedAt is when the step successfully completed execution.
	// Nil if the step has not completed.
	ExecutedAt *time.Time

	// CompensatedAt is when the step successfully completed compensation.
	// Nil if the step has not been compensated.
	CompensatedAt *time.Time

	// Error holds the last error message for this step.
	Error string
}

// SetData stores a JSON-serializable value under the given key.
func (st *SagaState) SetData(key string, value any) {
	if st.Data == nil {
		st.Data = make(map[string]any)
	}
	st.Data[key] = value
}

// GetData retrieves a raw value from the saga data map.
func (st *SagaState) GetData(key string) (any, bool) {
	if st.Data == nil {
		return nil, false
	}
	v, ok := st.Data[key]
	return v, ok
}

// GetDataAs retrieves a value and unmarshals it into the target via JSON
// round-trip. This handles the case where Data was deserialized from JSON
// and nested values are json.RawMessage.
func (st *SagaState) GetDataAs(key string, target any) error {
	if st.Data == nil {
		return nil
	}
	v, ok := st.Data[key]
	if !ok {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

// AppendOutbox adds an outbox record to the saga's pending buffer.
// The orchestrator flushes buffered records atomically with the next
// step state update.
func (st *SagaState) AppendOutbox(rec OutboxRecord) {
	st.outboxBuffer = append(st.outboxBuffer, rec)
}

// flushOutbox returns and clears the pending outbox buffer.
func (st *SagaState) flushOutbox() []OutboxRecord {
	recs := st.outboxBuffer
	st.outboxBuffer = nil
	return recs
}

// SagaStore persists saga state for crash recovery.
// All methods must be safe for concurrent use.
type SagaStore interface {
	// Save creates or replaces a saga state record.
	Save(ctx context.Context, st *SagaState) error

	// Load retrieves a saga state by ID.
	Load(ctx context.Context, sagaID string) (*SagaState, error)

	// UpdateStepAndOutbox atomically updates a step's state and appends
	// outbox records. This is the atomic outbox pattern: step state +
	// event records in one transaction.
	UpdateStepAndOutbox(ctx context.Context, sagaID string, stepIdx int, step StepState, outbox []OutboxRecord) error

	// ListIncomplete returns all sagas with status Running or
	// Compensating — the set that needs resume after a crash.
	ListIncomplete(ctx context.Context) ([]*SagaState, error)

	// ClaimSaga atomically marks a saga as being resumed by this
	// instance. Returns true if this caller won the claim, false if
	// another instance already claimed it. Prevents concurrent resume
	// of the same saga across multiple service replicas.
	ClaimSaga(ctx context.Context, sagaID string, claimedBy string) (bool, error)

	// ClearClaim releases the claim on a saga after resume completes.
	ClearClaim(ctx context.Context, sagaID string) error
}

// OutboxStore persists outbox records for reliable event delivery.
// It is a separate interface from SagaStore; a single concrete store
// can implement both.
type OutboxStore interface {
	// AppendOutbox adds a record to the outbox table.
	AppendOutbox(ctx context.Context, rec OutboxRecord) error

	// ListPendingOutbox returns up to limit pending outbox records.
	ListPendingOutbox(ctx context.Context, limit int) ([]OutboxRecord, error)

	// MarkPublished marks a record as successfully published.
	MarkPublished(ctx context.Context, id string) error

	// MarkFailed increments the fail count and, if the threshold is
	// reached, marks the record as Failed. The error message is stored.
	// threshold <= 0 means never mark as Failed (keep retrying).
	MarkFailed(ctx context.Context, id string, errMsg string, threshold int) error
}
