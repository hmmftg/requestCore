package saga

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hmmftg/requestCore/v2/telemetry"
	"github.com/hmmftg/requestCore/v2/workers"
)

// Step represents a single local transaction in a saga. Each step has
// an Execute function that performs the forward action and a Compensate
// function that undoes the action if a later step fails.
//
// Both functions receive the saga context and the shared SagaState,
// which can be used to pass data between steps via SetData/GetData.
type Step struct {
	// Name identifies the step within the saga. Must be unique.
	Name string

	// Execute performs the forward action. If it returns an error,
	// the saga stops forward execution and begins compensation.
	Execute func(ctx context.Context, st *SagaState) error

	// Compensate undoes the forward action. It is called in reverse
	// order for all steps that completed successfully. If nil, the
	// step has no compensation (e.g. a read-only step).
	Compensate func(ctx context.Context, st *SagaState) error
}

// Saga is a sequence of Steps executed in order with reverse
// compensation on failure.
type Saga struct {
	// ID is the unique identifier for this saga execution.
	ID string

	// Name is the human-readable name of the saga definition.
	Name string

	// Steps is the ordered list of steps to execute.
	Steps []Step
}

// Orchestrator runs saga executions with telemetry and optional durable
// state persistence for crash recovery.
type Orchestrator struct {
	store    SagaStore
	sink     telemetry.Sink
	timeout  time.Duration
	clock    func() time.Time
	registry *SagaRegistry
}

// OrchestratorOption configures an Orchestrator at construction time.
type OrchestratorOption func(*Orchestrator)

// WithStore sets the saga state store for persistence and crash recovery.
// If nil (default), sagas run stateless and cannot be resumed after crashes.
func WithStore(s SagaStore) OrchestratorOption {
	return func(o *Orchestrator) { o.store = s }
}

// WithSink sets the telemetry sink for recording saga lifecycle events.
// If nil (default), telemetry.NopSink is used.
func WithSink(s telemetry.Sink) OrchestratorOption {
	return func(o *Orchestrator) { o.sink = s }
}

// WithTimeout sets a per-saga execution timeout. If the timeout elapses,
// the context is cancelled and compensation begins. Zero means no timeout.
func WithTimeout(d time.Duration) OrchestratorOption {
	return func(o *Orchestrator) { o.timeout = d }
}

// WithRegistry sets the saga registry for looking up step definitions
// during crash recovery. Without a registry, ResumeAll can only skip
// already-completed steps but cannot execute or compensate pending
// steps because the real step functions are not available.
func WithRegistry(r *SagaRegistry) OrchestratorOption {
	return func(o *Orchestrator) { o.registry = r }
}

// withClock sets the clock source for deterministic testing.
func withClock(clock func() time.Time) OrchestratorOption {
	return func(o *Orchestrator) { o.clock = clock }
}

// NewOrchestrator creates an Orchestrator with the given options.
func NewOrchestrator(opts ...OrchestratorOption) *Orchestrator {
	o := &Orchestrator{
		sink:  telemetry.NopSink{},
		clock: time.Now,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Execute runs the saga's steps in order. On step failure, it
// compensates completed steps in reverse order. If a store is
// configured, saga state is persisted for crash recovery.
func (o *Orchestrator) Execute(ctx context.Context, saga *Saga) error {
	return o.executeWithState(ctx, saga, nil)
}

// ResumeAll loads all incomplete sagas from the store and resumes them
// from their last persisted step state. This is intended to be called
// on startup or periodically via a workers.ScheduledJob.
//
// If a SagaRegistry is configured, step definitions (Execute/Compensate
// functions) are looked up by saga name. Without a registry, only
// already-completed steps can be skipped; pending steps cannot be
// executed because the real functions are unavailable.
func (o *Orchestrator) ResumeAll(ctx context.Context) error {
	if o.store == nil {
		return nil
	}

	incomplete, err := o.store.ListIncomplete(ctx)
	if err != nil {
		return fmt.Errorf("saga: list incomplete: %w", err)
	}

	claimedBy := fmt.Sprintf("orchestrator-%p", o)

	for _, st := range incomplete {
		claimed, err := o.store.ClaimSaga(ctx, st.ID, claimedBy)
		if err != nil {
			return fmt.Errorf("saga: claim %s: %w", st.ID, err)
		}
		if !claimed {
			continue
		}

		saga := o.reconstructSaga(st)

		_ = o.executeWithState(ctx, saga, st)

		_ = o.store.ClearClaim(ctx, st.ID)
	}

	return nil
}

// reconstructSaga builds a Saga from persisted state. If a registry
// is configured, it looks up the real step definitions by saga name
// and merges them with the persisted step names. Without a registry,
// steps have nil Execute/Compensate functions, meaning only already-
// completed steps can be skipped during resume.
func (o *Orchestrator) reconstructSaga(st *SagaState) *Saga {
	saga := &Saga{
		ID:   st.ID,
		Name: st.SagaName,
	}

	if o.registry != nil {
		if def := o.registry.Lookup(st.SagaName); def != nil {
			saga.Steps = make([]Step, len(st.Steps))
			for i, ss := range st.Steps {
				found := false
				for _, defStep := range def.Steps {
					if defStep.Name == ss.Name {
						saga.Steps[i] = defStep
						found = true
						break
					}
				}
				if !found {
					saga.Steps[i] = Step{Name: ss.Name}
				}
			}
			return saga
		}
	}

	saga.Steps = make([]Step, len(st.Steps))
	for i, ss := range st.Steps {
		saga.Steps[i] = Step{Name: ss.Name}
	}
	return saga
}

// executeWithState runs the saga execution loop. If resumeState is
// non-nil, it resumes from the persisted state (skipping completed
// steps and compensating if needed).
func (o *Orchestrator) executeWithState(ctx context.Context, saga *Saga, resumeState *SagaState) error {
	now := o.clock()

	var st *SagaState
	if resumeState != nil {
		st = resumeState
	} else {
		st = &SagaState{
			ID:        saga.ID,
			SagaName:  saga.Name,
			Status:    SagaRunning,
			StartedAt: now,
			UpdatedAt: now,
			Steps:     make([]StepState, len(saga.Steps)),
		}
		for i, step := range saga.Steps {
			if st.Steps[i].Name == "" {
				st.Steps[i] = StepState{
					Name:           step.Name,
					Status:         StepPending,
					IdempotencyKey: MakeIdempotencyKey(saga.ID, step.Name, "execute"),
				}
			}
		}
	}

	if o.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.timeout)
		defer cancel()
	}

	o.recordEvent(telemetry.EventStart, "saga-"+saga.ID+"-req", 0, nil, st)

	if o.store != nil && resumeState == nil {
		if err := o.store.Save(ctx, st); err != nil {
			return fmt.Errorf("saga: save state: %w", err)
		}
	}

	// Forward execution.
	var failedStepIdx int = -1
	for i, step := range saga.Steps {
		if ctx.Err() != nil {
			failedStepIdx = i
			break
		}

		if st.Steps[i].Status == StepDone {
			continue
		}

		st.Steps[i].Status = StepExecuting
		st.Steps[i].Attempt++
		st.UpdatedAt = o.clock()

		start := o.clock()
		err := o.runStep(ctx, step.Execute, st)
		elapsed := o.clock().Sub(start)

		if err != nil {
			st.Steps[i].Status = StepFailed
			st.Steps[i].Error = err.Error()
			st.UpdatedAt = o.clock()

			o.recordEvent(telemetry.EventFailure, o.stepOp(saga.ID, step.Name, "req-failed"), elapsed, err, st)

			if o.store != nil {
				_ = o.store.UpdateStepAndOutbox(ctx, saga.ID, i, st.Steps[i], st.flushOutbox())
			}

			failedStepIdx = i
			break
		}

		executedAt := o.clock()
		st.Steps[i].Status = StepDone
		st.Steps[i].Error = ""
		st.Steps[i].ExecutedAt = &executedAt
		st.UpdatedAt = o.clock()

		o.recordEvent(telemetry.EventSuccess, o.stepOp(saga.ID, step.Name, "req"), elapsed, nil, st)

		if o.store != nil {
			if storeErr := o.store.UpdateStepAndOutbox(ctx, saga.ID, i, st.Steps[i], st.flushOutbox()); storeErr != nil {
				return fmt.Errorf("saga: persist step %s: %w", step.Name, storeErr)
			}
		}
	}

	if failedStepIdx >= 0 {
		st.Status = SagaCompensating
		st.UpdatedAt = o.clock()
		if o.store != nil {
			_ = o.store.Save(ctx, st)
		}

		o.compensate(ctx, saga, st, failedStepIdx)

		if o.allCompensated(st, failedStepIdx) {
			st.Status = SagaCompensated
		} else {
			st.Status = SagaFailed
		}
		st.UpdatedAt = o.clock()
		if o.store != nil {
			_ = o.store.Save(ctx, st)
		}

		err := ctx.Err()
		if err == nil && failedStepIdx < len(saga.Steps) {
			err = fmt.Errorf("saga %s failed at step %s", saga.ID, saga.Steps[failedStepIdx].Name)
		} else if err != nil {
			err = fmt.Errorf("saga %s cancelled: %w", saga.ID, err)
		}

		o.recordEvent(telemetry.EventFailure, "saga-"+saga.ID+"-req", 0, err, st)
		return err
	}

	st.Status = SagaCompleted
	st.UpdatedAt = o.clock()
	if o.store != nil {
		_ = o.store.Save(ctx, st)
	}

	o.recordEvent(telemetry.EventSuccess, "saga-"+saga.ID+"-req", 0, nil, st)
	return nil
}

// compensate runs compensation for all completed steps before
// failedStepIdx, in reverse order.
func (o *Orchestrator) compensate(ctx context.Context, saga *Saga, st *SagaState, failedStepIdx int) {
	for i := failedStepIdx - 1; i >= 0; i-- {
		step := saga.Steps[i]

		if step.Compensate == nil {
			continue
		}

		if st.Steps[i].Status == StepCompensated {
			continue
		}

		if st.Steps[i].Status != StepDone {
			continue
		}

		st.Steps[i].Status = StepCompensating
		st.Steps[i].IdempotencyKey = MakeIdempotencyKey(saga.ID, step.Name, "compensate")
		st.UpdatedAt = o.clock()

		start := o.clock()
		err := o.runStep(ctx, step.Compensate, st)
		elapsed := o.clock().Sub(start)

		if err != nil {
			st.Steps[i].Status = StepFailed
			st.Steps[i].Error = err.Error()
			st.UpdatedAt = o.clock()

			o.recordEvent(telemetry.EventFailure, o.stepOp(saga.ID, step.Name, "compensate-req-failed"), elapsed, err, st)

			if o.store != nil {
				_ = o.store.UpdateStepAndOutbox(ctx, saga.ID, i, st.Steps[i], st.flushOutbox())
			}
			continue
		}

		compensatedAt := o.clock()
		st.Steps[i].Status = StepCompensated
		st.Steps[i].Error = ""
		st.Steps[i].CompensatedAt = &compensatedAt
		st.UpdatedAt = o.clock()

		o.recordEvent(telemetry.EventSuccess, o.stepOp(saga.ID, step.Name, "compensate-req"), elapsed, nil, st)

		if o.store != nil {
			_ = o.store.UpdateStepAndOutbox(ctx, saga.ID, i, st.Steps[i], st.flushOutbox())
		}
	}
}

// allCompensated returns true if all steps before the failed step have
// been compensated. Steps at or after the failed index are not checked
// since they either never executed or are the failed step itself.
func (o *Orchestrator) allCompensated(st *SagaState, failedStepIdx int) bool {
	for i := 0; i < failedStepIdx; i++ {
		if st.Steps[i].Status == StepCompensating || st.Steps[i].Status == StepFailed {
			return false
		}
	}
	return true
}

// runStep executes a step function with panic recovery.
func (o *Orchestrator) runStep(ctx context.Context, fn func(context.Context, *SagaState) error, st *SagaState) (err error) {
	if fn == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("saga: step panicked: %v", r)
		}
	}()
	return fn(ctx, st)
}

// stepOp builds the telemetry operation string for a step event.
func (o *Orchestrator) stepOp(sagaID, stepName, suffix string) string {
	return "saga-" + sagaID + "-" + stepName + "-" + suffix
}

// recordEvent emits a telemetry event through the sink.
func (o *Orchestrator) recordEvent(eventType telemetry.EventType, operation string, duration time.Duration, err error, st *SagaState) {
	if o.sink == nil {
		return
	}

	attrs := []slog.Attr{
		slog.String("saga_id", st.ID),
		slog.String("saga_name", st.SagaName),
		slog.String("saga_status", string(st.Status)),
	}

	o.sink.Record(telemetry.Event{
		Type:      eventType,
		Operation: operation,
		Duration:  duration,
		Err:       err,
		Attrs:     attrs,
	})
}

// ScheduledResumeJob returns a workers.ScheduledJob that calls
// ResumeAll at the given interval. Register it on a workers.Scheduler
// to enable automatic crash recovery.
func ScheduledResumeJob(orch *Orchestrator, interval time.Duration) workers.ScheduledJob {
	return workers.ScheduledJob{
		Name:     "saga-resume",
		Handler:  func(jctx *workers.JobContext) error { return orch.ResumeAll(jctx.Context) },
		Interval: interval,
		Options: workers.JobOptions{
			MaxAttempts: 1,
		},
	}
}
