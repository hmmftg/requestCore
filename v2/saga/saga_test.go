package saga

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/v2/telemetry"
)

// recordingSink collects telemetry events for test verification.
type recordingSink struct {
	mu     sync.Mutex
	events []telemetry.Event
}

func (s *recordingSink) Record(e telemetry.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
}

func (s *recordingSink) Events() []telemetry.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]telemetry.Event(nil), s.events...)
}

func (s *recordingSink) CountByType(typ telemetry.EventType) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, e := range s.events {
		if e.Type == typ {
			count++
		}
	}
	return count
}

func (s *recordingSink) FindOperation(op string) (telemetry.Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.events {
		if e.Operation == op {
			return e, true
		}
	}
	return telemetry.Event{}, false
}

func TestSaga_HappyPath(t *testing.T) {
	sink := &recordingSink{}
	orch := NewOrchestrator(WithSink(sink))

	var execOrder []string
	s := &Saga{
		ID:   "saga-1",
		Name: "test-happy",
		Steps: []Step{
			{Name: "step-1", Execute: func(_ context.Context, _ *SagaState) error {
				execOrder = append(execOrder, "step-1")
				return nil
			}},
			{Name: "step-2", Execute: func(_ context.Context, _ *SagaState) error {
				execOrder = append(execOrder, "step-2")
				return nil
			}},
			{Name: "step-3", Execute: func(_ context.Context, _ *SagaState) error {
				execOrder = append(execOrder, "step-3")
				return nil
			}},
		},
	}

	err := orch.Execute(context.Background(), s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(execOrder) != 3 {
		t.Fatalf("expected 3 steps executed, got %d", len(execOrder))
	}
	if execOrder[0] != "step-1" || execOrder[1] != "step-2" || execOrder[2] != "step-3" {
		t.Fatalf("unexpected execution order: %v", execOrder)
	}

	if sink.CountByType(telemetry.EventSuccess) < 4 {
		t.Errorf("expected at least 4 success events (3 steps + 1 saga), got %d", sink.CountByType(telemetry.EventSuccess))
	}
	if sink.CountByType(telemetry.EventFailure) != 0 {
		t.Errorf("expected 0 failure events, got %d", sink.CountByType(telemetry.EventFailure))
	}
}

func TestSaga_StepFailureTriggersCompensation(t *testing.T) {
	sink := &recordingSink{}
	orch := NewOrchestrator(WithSink(sink))

	var execOrder []string
	var compOrder []string

	s := &Saga{
		ID:   "saga-2",
		Name: "test-compensate",
		Steps: []Step{
			{
				Name: "step-1",
				Execute: func(_ context.Context, _ *SagaState) error {
					execOrder = append(execOrder, "step-1")
					return nil
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					compOrder = append(compOrder, "step-1")
					return nil
				},
			},
			{
				Name: "step-2",
				Execute: func(_ context.Context, _ *SagaState) error {
					execOrder = append(execOrder, "step-2")
					return nil
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					compOrder = append(compOrder, "step-2")
					return nil
				},
			},
			{
				Name: "step-3",
				Execute: func(_ context.Context, _ *SagaState) error {
					execOrder = append(execOrder, "step-3")
					return errors.New("step-3 failed")
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					compOrder = append(compOrder, "step-3")
					return nil
				},
			},
		},
	}

	err := orch.Execute(context.Background(), s)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if len(execOrder) != 3 {
		t.Fatalf("expected 3 steps executed, got %d", len(execOrder))
	}
	if len(compOrder) != 2 {
		t.Fatalf("expected 2 compensations (step-2, step-1), got %d", len(compOrder))
	}
	if compOrder[0] != "step-2" || compOrder[1] != "step-1" {
		t.Fatalf("expected reverse compensation order [step-2, step-1], got %v", compOrder)
	}

	if _, found := sink.FindOperation("saga-saga-2-step-3-req-failed"); !found {
		t.Errorf("expected step-3 failure event")
	}
	if _, found := sink.FindOperation("saga-saga-2-step-1-compensate-req"); !found {
		t.Errorf("expected step-1 compensate success event")
	}
	if _, found := sink.FindOperation("saga-saga-2-step-2-compensate-req"); !found {
		t.Errorf("expected step-2 compensate success event")
	}
}

func TestSaga_ContextCancellation(t *testing.T) {
	sink := &recordingSink{}
	orch := NewOrchestrator(WithSink(sink))

	var compOrder []string

	ctx, cancel := context.WithCancel(context.Background())

	s := &Saga{
		ID:   "saga-3",
		Name: "test-cancel",
		Steps: []Step{
			{
				Name: "step-1",
				Execute: func(_ context.Context, _ *SagaState) error {
					return nil
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					compOrder = append(compOrder, "step-1")
					return nil
				},
			},
			{
				Name: "step-2",
				Execute: func(ctx context.Context, _ *SagaState) error {
					cancel()
					<-ctx.Done()
					return ctx.Err()
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					compOrder = append(compOrder, "step-2")
					return nil
				},
			},
		},
	}

	err := orch.Execute(ctx, s)
	if err == nil {
		t.Fatal("expected error from cancelled saga, got nil")
	}

	if len(compOrder) != 1 {
		t.Fatalf("expected 1 compensation (step-1), got %d", len(compOrder))
	}
	if compOrder[0] != "step-1" {
		t.Fatalf("expected step-1 compensation, got %v", compOrder)
	}
}

func TestSaga_TimeoutTriggersCompensation(t *testing.T) {
	sink := &recordingSink{}
	orch := NewOrchestrator(
		WithSink(sink),
		WithTimeout(50*time.Millisecond),
	)

	var compOrder []string

	s := &Saga{
		ID:   "saga-4",
		Name: "test-timeout",
		Steps: []Step{
			{
				Name: "step-1",
				Execute: func(_ context.Context, _ *SagaState) error {
					return nil
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					compOrder = append(compOrder, "step-1")
					return nil
				},
			},
			{
				Name: "step-2",
				Execute: func(ctx context.Context, _ *SagaState) error {
					select {
					case <-time.After(200 * time.Millisecond):
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					compOrder = append(compOrder, "step-2")
					return nil
				},
			},
		},
	}

	err := orch.Execute(context.Background(), s)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if len(compOrder) != 1 {
		t.Fatalf("expected 1 compensation (step-1), got %d", len(compOrder))
	}
}

func TestSaga_CompensationFailureDoesNotAbortRemaining(t *testing.T) {
	sink := &recordingSink{}
	orch := NewOrchestrator(WithSink(sink))

	var compOrder []string

	s := &Saga{
		ID:   "saga-5",
		Name: "test-comp-failure",
		Steps: []Step{
			{
				Name: "step-1",
				Execute: func(_ context.Context, _ *SagaState) error {
					return nil
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					compOrder = append(compOrder, "step-1")
					return nil
				},
			},
			{
				Name: "step-2",
				Execute: func(_ context.Context, _ *SagaState) error {
					return nil
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					compOrder = append(compOrder, "step-2")
					return errors.New("compensation failed")
				},
			},
			{
				Name: "step-3",
				Execute: func(_ context.Context, _ *SagaState) error {
					return errors.New("step-3 failed")
				},
			},
		},
	}

	err := orch.Execute(context.Background(), s)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if len(compOrder) != 2 {
		t.Fatalf("expected 2 compensation attempts, got %d", len(compOrder))
	}
	if compOrder[0] != "step-2" || compOrder[1] != "step-1" {
		t.Fatalf("expected [step-2, step-1], got %v", compOrder)
	}

	if _, found := sink.FindOperation("saga-saga-5-step-2-compensate-req-failed"); !found {
		t.Errorf("expected step-2 compensate failure event")
	}
	if _, found := sink.FindOperation("saga-saga-5-step-1-compensate-req"); !found {
		t.Errorf("expected step-1 compensate success event")
	}
}

func TestSaga_StatelessMode(t *testing.T) {
	sink := &recordingSink{}
	orch := NewOrchestrator(WithSink(sink))

	s := &Saga{
		ID:   "saga-6",
		Name: "test-stateless",
		Steps: []Step{
			{Name: "step-1", Execute: func(_ context.Context, st *SagaState) error {
				st.SetData("key", "value")
				return nil
			}},
			{Name: "step-2", Execute: func(_ context.Context, st *SagaState) error {
				val, ok := st.GetData("key")
				if !ok || val != "value" {
					return errors.New("expected key=value from step-1")
				}
				return nil
			}},
		},
	}

	err := orch.Execute(context.Background(), s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaga_StatefulMode(t *testing.T) {
	sink := &recordingSink{}
	store := NewMemoryStore()
	orch := NewOrchestrator(WithSink(sink), WithStore(store))

	s := &Saga{
		ID:   "saga-7",
		Name: "test-stateful",
		Steps: []Step{
			{Name: "step-1", Execute: func(_ context.Context, _ *SagaState) error {
				return nil
			}},
			{Name: "step-2", Execute: func(_ context.Context, _ *SagaState) error {
				return nil
			}},
		},
	}

	err := orch.Execute(context.Background(), s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	st, err := store.Load(context.Background(), "saga-7")
	if err != nil {
		t.Fatalf("failed to load saga state: %v", err)
	}
	if st.Status != SagaCompleted {
		t.Errorf("expected status Completed, got %s", st.Status)
	}
	if len(st.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(st.Steps))
	}
	if st.Steps[0].Status != StepDone || st.Steps[1].Status != StepDone {
		t.Errorf("expected both steps Done, got %s, %s", st.Steps[0].Status, st.Steps[1].Status)
	}
}

func TestSaga_StatefulCompensation(t *testing.T) {
	sink := &recordingSink{}
	store := NewMemoryStore()
	orch := NewOrchestrator(WithSink(sink), WithStore(store))

	s := &Saga{
		ID:   "saga-8",
		Name: "test-stateful-comp",
		Steps: []Step{
			{
				Name: "step-1",
				Execute: func(_ context.Context, _ *SagaState) error {
					return nil
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					return nil
				},
			},
			{
				Name: "step-2",
				Execute: func(_ context.Context, _ *SagaState) error {
					return errors.New("step-2 failed")
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					return nil
				},
			},
		},
	}

	err := orch.Execute(context.Background(), s)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, err := store.Load(context.Background(), "saga-8")
	if err != nil {
		t.Fatalf("failed to load saga state: %v", err)
	}
	if st.Status != SagaCompensated {
		t.Errorf("expected status Compensated, got %s", st.Status)
	}
	if st.Steps[0].Status != StepCompensated {
		t.Errorf("expected step-1 Compensated, got %s", st.Steps[0].Status)
	}
	if st.Steps[1].Status != StepFailed {
		t.Errorf("expected step-2 Failed, got %s", st.Steps[1].Status)
	}
}

func TestSaga_StepPanicRecovery(t *testing.T) {
	sink := &recordingSink{}
	orch := NewOrchestrator(WithSink(sink))

	s := &Saga{
		ID:   "saga-9",
		Name: "test-panic",
		Steps: []Step{
			{
				Name: "step-1",
				Execute: func(_ context.Context, _ *SagaState) error {
					return nil
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					return nil
				},
			},
			{
				Name: "step-2",
				Execute: func(_ context.Context, _ *SagaState) error {
					panic("boom")
				},
			},
		},
	}

	err := orch.Execute(context.Background(), s)
	if err == nil {
		t.Fatal("expected error from panic, got nil")
	}
}
