package saga

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestSagaRegistry_RegisterAndLookup(t *testing.T) {
	reg := NewSagaRegistry()

	s1 := &Saga{Name: "checkout", Steps: []Step{{Name: "step-1"}}}
	if !reg.Register(s1) {
		t.Fatal("expected first register to succeed")
	}

	if reg.Register(s1) {
		t.Fatal("expected duplicate register to fail")
	}

	found := reg.Lookup("checkout")
	if found == nil {
		t.Fatal("expected to find registered saga")
	}
	if found.Name != "checkout" {
		t.Errorf("expected name checkout, got %s", found.Name)
	}

	if reg.Lookup("nonexistent") != nil {
		t.Error("expected nil for unregistered saga")
	}
}

func TestSagaRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewSagaRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			reg.Register(&Saga{Name: "saga-" + string(rune(n))})
			reg.Lookup("saga-" + string(rune(n)))
		}(i)
	}
	wg.Wait()
}

func TestOrchestrator_ResumeWithRegistry(t *testing.T) {
	store := NewMemoryStore()
	reg := NewSagaRegistry()

	var execCount, compCount int

	sagaDef := &Saga{
		Name: "resumable",
		Steps: []Step{
			{
				Name: "step-1",
				Execute: func(_ context.Context, _ *SagaState) error {
					execCount++
					return nil
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					compCount++
					return nil
				},
			},
			{
				Name: "step-2",
				Execute: func(_ context.Context, _ *SagaState) error {
					execCount++
					return errors.New("step-2 fails on resume")
				},
				Compensate: func(_ context.Context, _ *SagaState) error {
					compCount++
					return nil
				},
			},
		},
	}
	reg.Register(sagaDef)

	orch := NewOrchestrator(WithStore(store), WithRegistry(reg))

	st := &SagaState{
		ID:       "saga-resume-1",
		SagaName: "resumable",
		Status:   SagaRunning,
		Steps: []StepState{
			{Name: "step-1", Status: StepDone},
			{Name: "step-2", Status: StepPending},
		},
	}
	_ = store.Save(context.Background(), st)

	_ = orch.ResumeAll(context.Background())

	if execCount != 1 {
		t.Errorf("expected 1 execution (step-2), got %d", execCount)
	}
	if compCount != 1 {
		t.Errorf("expected 1 compensation (step-1), got %d", compCount)
	}

	final, _ := store.Load(context.Background(), "saga-resume-1")
	if final.Status != SagaCompensated {
		t.Errorf("expected status Compensated, got %s", final.Status)
	}
}

func TestOrchestrator_ResumeWithoutRegistry_SkipsCompleted(t *testing.T) {
	store := NewMemoryStore()

	orch := NewOrchestrator(WithStore(store))

	st := &SagaState{
		ID:       "saga-resume-2",
		SagaName: "no-registry",
		Status:   SagaRunning,
		Steps: []StepState{
			{Name: "step-1", Status: StepDone},
			{Name: "step-2", Status: StepDone},
		},
	}
	_ = store.Save(context.Background(), st)

	_ = orch.ResumeAll(context.Background())

	final, _ := store.Load(context.Background(), "saga-resume-2")
	if final.Status != SagaCompleted {
		t.Errorf("expected status Completed (all steps already done), got %s", final.Status)
	}
}

func TestMemoryStore_DeepCopyIsolation(t *testing.T) {
	store := NewMemoryStore()

	st := &SagaState{
		ID:       "saga-copy-test",
		SagaName: "test",
		Status:   SagaRunning,
		Steps:    []StepState{{Name: "step-1", Status: StepPending}},
		Data:     map[string]any{"key": "value"},
	}
	_ = store.Save(context.Background(), st)

	st.Steps[0].Status = StepDone
	st.Data["key"] = "mutated"

	loaded, _ := store.Load(context.Background(), "saga-copy-test")
	if loaded.Steps[0].Status != StepPending {
		t.Errorf("deep copy failed: expected StepPending, got %s (original was mutated after Save)", loaded.Steps[0].Status)
	}
	if loaded.Data["key"] != "value" {
		t.Errorf("deep copy failed: expected value, got %v (original was mutated after Save)", loaded.Data["key"])
	}

	loaded.Steps[0].Status = StepFailed
	loaded.Data["key"] = "mutated2"

	reloaded, _ := store.Load(context.Background(), "saga-copy-test")
	if reloaded.Steps[0].Status != StepPending {
		t.Errorf("deep copy failed on Load: expected StepPending, got %s (returned copy was mutated)", reloaded.Steps[0].Status)
	}
	if reloaded.Data["key"] != "value" {
		t.Errorf("deep copy failed on Load: expected value, got %v (returned copy was mutated)", reloaded.Data["key"])
	}
}
