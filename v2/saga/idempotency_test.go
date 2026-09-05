package saga

import (
	"context"
	"testing"
)

func TestMakeIdempotencyKey(t *testing.T) {
	key := MakeIdempotencyKey("saga-1", "step-1", "execute")
	if key.String() != "saga-1:step-1:execute" {
		t.Errorf("expected saga-1:step-1:execute, got %s", key.String())
	}
}

func TestSagaState_IsStepExecuted(t *testing.T) {
	st := &SagaState{
		Steps: []StepState{
			{Name: "step-1", Status: StepDone},
			{Name: "step-2", Status: StepPending},
		},
	}

	if !st.IsStepExecuted("step-1") {
		t.Error("expected step-1 to be executed")
	}
	if st.IsStepExecuted("step-2") {
		t.Error("expected step-2 to NOT be executed")
	}
	if st.IsStepExecuted("step-3") {
		t.Error("expected step-3 (nonexistent) to NOT be executed")
	}
}

func TestSagaState_IsStepCompensated(t *testing.T) {
	st := &SagaState{
		Steps: []StepState{
			{Name: "step-1", Status: StepCompensated},
			{Name: "step-2", Status: StepDone},
		},
	}

	if !st.IsStepCompensated("step-1") {
		t.Error("expected step-1 to be compensated")
	}
	if st.IsStepCompensated("step-2") {
		t.Error("expected step-2 to NOT be compensated")
	}
}

func TestSagaState_StepIndex(t *testing.T) {
	st := &SagaState{
		Steps: []StepState{
			{Name: "step-1"},
			{Name: "step-2"},
			{Name: "step-3"},
		},
	}

	if idx := st.StepIndex("step-2"); idx != 1 {
		t.Errorf("expected index 1 for step-2, got %d", idx)
	}
	if idx := st.StepIndex("nonexistent"); idx != -1 {
		t.Errorf("expected -1 for nonexistent step, got %d", idx)
	}
}

func TestSagaState_SetGetData(t *testing.T) {
	st := &SagaState{}

	st.SetData("key", "value")
	val, ok := st.GetData("key")
	if !ok || val != "value" {
		t.Errorf("expected key=value, got %v, ok=%v", val, ok)
	}

	_, ok = st.GetData("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent key")
	}
}

func TestSagaState_GetDataAs(t *testing.T) {
	st := &SagaState{}

	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	st.SetData("person", map[string]any{"name": "Alice", "age": 30})

	var p Person
	if err := st.GetDataAs("person", &p); err != nil {
		t.Fatalf("GetDataAs: %v", err)
	}
	if p.Name != "Alice" || p.Age != 30 {
		t.Errorf("expected Alice/30, got %s/%d", p.Name, p.Age)
	}

	if err := st.GetDataAs("nonexistent", &p); err != nil {
		t.Errorf("expected nil error for nonexistent key, got %v", err)
	}
}

func TestSagaState_AppendOutbox(t *testing.T) {
	st := &SagaState{}

	rec1 := OutboxRecord{ID: "ob-1", SagaID: "s1"}
	rec2 := OutboxRecord{ID: "ob-2", SagaID: "s1"}

	st.AppendOutbox(rec1)
	st.AppendOutbox(rec2)

	flushed := st.flushOutbox()
	if len(flushed) != 2 {
		t.Fatalf("expected 2 flushed records, got %d", len(flushed))
	}
	if flushed[0].ID != "ob-1" || flushed[1].ID != "ob-2" {
		t.Errorf("unexpected flush order: %v", flushed)
	}

	secondFlush := st.flushOutbox()
	if len(secondFlush) != 0 {
		t.Errorf("expected 0 records on second flush, got %d", len(secondFlush))
	}
}

func TestOrchestrator_IdempotentStepSkip(t *testing.T) {
	sink := &recordingSink{}
	store := NewMemoryStore()
	orch := NewOrchestrator(WithSink(sink), WithStore(store))

	execCount := 0
	s := &Saga{
		ID:   "saga-idem",
		Name: "test-idem",
		Steps: []Step{
			{
				Name: "step-1",
				Execute: func(_ context.Context, _ *SagaState) error {
					execCount++
					return nil
				},
			},
		},
	}

	if err := orch.Execute(context.Background(), s); err != nil {
		t.Fatalf("first execute: %v", err)
	}
	if execCount != 1 {
		t.Fatalf("expected 1 execution, got %d", execCount)
	}

	st, _ := store.Load(context.Background(), "saga-idem")
	if st.Steps[0].Status != StepDone {
		t.Fatalf("expected step Done, got %s", st.Steps[0].Status)
	}

	if err := orch.Execute(context.Background(), s); err != nil {
		t.Fatalf("second execute: %v", err)
	}
}
