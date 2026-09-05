package saga

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore_SaveLoad(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now()
	st := &SagaState{
		ID:        "saga-1",
		SagaName:  "test",
		Status:    SagaRunning,
		StartedAt: now,
		UpdatedAt: now,
		Steps: []StepState{
			{Name: "step-1", Status: StepDone, IdempotencyKey: "saga-1:step-1:execute"},
		},
	}

	if err := store.Save(context.Background(), st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(context.Background(), "saga-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID != "saga-1" {
		t.Errorf("expected ID saga-1, got %s", loaded.ID)
	}
	if loaded.Status != SagaRunning {
		t.Errorf("expected status Running, got %s", loaded.Status)
	}
	if len(loaded.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(loaded.Steps))
	}
	if loaded.Steps[0].Status != StepDone {
		t.Errorf("expected step Done, got %s", loaded.Steps[0].Status)
	}
}

func TestMemoryStore_LoadNotFound(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.Load(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent saga, got nil")
	}
}

func TestMemoryStore_UpdateStepAndOutbox(t *testing.T) {
	store := NewMemoryStore()
	st := &SagaState{
		ID:       "saga-2",
		SagaName: "test",
		Status:   SagaRunning,
		Steps:    []StepState{{Name: "step-1", Status: StepPending}},
	}
	_ = store.Save(context.Background(), st)

	now := time.Now()
	updatedStep := StepState{
		Name:       "step-1",
		Status:     StepDone,
		Attempt:    1,
		ExecutedAt: &now,
	}
	outboxRecs := []OutboxRecord{
		{ID: "ob-1", SagaID: "saga-2", StepName: "step-1", EventType: "Created", Payload: []byte("{}"), CreatedAt: now},
	}

	if err := store.UpdateStepAndOutbox(context.Background(), "saga-2", 0, updatedStep, outboxRecs); err != nil {
		t.Fatalf("UpdateStepAndOutbox: %v", err)
	}

	loaded, _ := store.Load(context.Background(), "saga-2")
	if loaded.Steps[0].Status != StepDone {
		t.Errorf("expected step Done, got %s", loaded.Steps[0].Status)
	}

	recs, err := store.ListPendingOutbox(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListPendingOutbox: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 outbox record, got %d", len(recs))
	}
	if recs[0].ID != "ob-1" {
		t.Errorf("expected outbox ID ob-1, got %s", recs[0].ID)
	}
}

func TestMemoryStore_UpdateStepOutOfRange(t *testing.T) {
	store := NewMemoryStore()
	st := &SagaState{
		ID:       "saga-3",
		SagaName: "test",
		Status:   SagaRunning,
		Steps:    []StepState{{Name: "step-1"}},
	}
	_ = store.Save(context.Background(), st)

	err := store.UpdateStepAndOutbox(context.Background(), "saga-3", 5, StepState{}, nil)
	if err == nil {
		t.Fatal("expected error for out-of-range step index, got nil")
	}
}

func TestMemoryStore_ListIncomplete(t *testing.T) {
	store := NewMemoryStore()
	_ = store.Save(context.Background(), &SagaState{ID: "s1", SagaName: "t", Status: SagaRunning, Steps: []StepState{}})
	_ = store.Save(context.Background(), &SagaState{ID: "s2", SagaName: "t", Status: SagaCompleted, Steps: []StepState{}})
	_ = store.Save(context.Background(), &SagaState{ID: "s3", SagaName: "t", Status: SagaCompensating, Steps: []StepState{}})
	_ = store.Save(context.Background(), &SagaState{ID: "s4", SagaName: "t", Status: SagaFailed, Steps: []StepState{}})

	incomplete, err := store.ListIncomplete(context.Background())
	if err != nil {
		t.Fatalf("ListIncomplete: %v", err)
	}
	if len(incomplete) != 2 {
		t.Fatalf("expected 2 incomplete sagas, got %d", len(incomplete))
	}
	ids := map[string]bool{}
	for _, s := range incomplete {
		ids[s.ID] = true
	}
	if !ids["s1"] || !ids["s3"] {
		t.Errorf("expected s1 and s3, got %v", ids)
	}
}

func TestMemoryStore_ClaimSaga(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Save(ctx, &SagaState{ID: "s1", SagaName: "t", Status: SagaRunning, Steps: []StepState{}})

	claimed1, err := store.ClaimSaga(ctx, "s1", "instance-1")
	if err != nil {
		t.Fatalf("ClaimSaga 1: %v", err)
	}
	if !claimed1 {
		t.Fatal("expected first claim to succeed")
	}

	claimed2, err := store.ClaimSaga(ctx, "s1", "instance-2")
	if err != nil {
		t.Fatalf("ClaimSaga 2: %v", err)
	}
	if claimed2 {
		t.Fatal("expected second claim from different instance to fail")
	}

	claimed3, err := store.ClaimSaga(ctx, "s1", "instance-1")
	if err != nil {
		t.Fatalf("ClaimSaga 3: %v", err)
	}
	if !claimed3 {
		t.Fatal("expected re-claim from same instance to succeed")
	}

	_ = store.ClearClaim(ctx, "s1")

	claimed4, err := store.ClaimSaga(ctx, "s1", "instance-2")
	if err != nil {
		t.Fatalf("ClaimSaga 4: %v", err)
	}
	if !claimed4 {
		t.Fatal("expected claim after clear to succeed")
	}
}

func TestMemoryStore_OutboxLifecycle(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	now := time.Now()

	rec := OutboxRecord{
		ID:        "ob-1",
		SagaID:    "s1",
		StepName:  "step-1",
		EventType: "Created",
		Payload:   []byte(`{"id":1}`),
		CreatedAt: now,
		Status:    OutboxPending,
	}

	if err := store.AppendOutbox(ctx, rec); err != nil {
		t.Fatalf("AppendOutbox: %v", err)
	}

	recs, err := store.ListPendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("ListPendingOutbox: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(recs))
	}

	if err := store.MarkPublished(ctx, "ob-1"); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}

	recs, _ = store.ListPendingOutbox(ctx, 10)
	if len(recs) != 0 {
		t.Fatalf("expected 0 pending after publish, got %d", len(recs))
	}

	rec2 := OutboxRecord{
		ID:        "ob-2",
		SagaID:    "s1",
		StepName:  "step-2",
		EventType: "Updated",
		Payload:   []byte(`{}`),
		CreatedAt: now,
		Status:    OutboxPending,
	}
	_ = store.AppendOutbox(ctx, rec2)

	for i := 0; i < 5; i++ {
		_ = store.MarkFailed(ctx, "ob-2", "publish error", 5)
	}

	recs, _ = store.ListPendingOutbox(ctx, 10)
	if len(recs) != 0 {
		t.Fatalf("expected 0 pending after 5 failures, got %d", len(recs))
	}
}

func TestMemoryStore_OutboxEmptyID(t *testing.T) {
	store := NewMemoryStore()
	err := store.AppendOutbox(context.Background(), OutboxRecord{})
	if err == nil {
		t.Fatal("expected error for empty outbox ID, got nil")
	}
}
