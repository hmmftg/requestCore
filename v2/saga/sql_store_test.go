//go:build sqlite

package saga

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newSQLiteStore(t *testing.T) *SQLStore {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	store, err := NewSQLStore(db, DialectSQLite)
	if err != nil {
		t.Fatalf("NewSQLStore: %v", err)
	}
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return store
}

func TestSQLStore_SaveLoad(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	st := &SagaState{
		ID:        "saga-sql-1",
		SagaName:  "test",
		Status:    SagaRunning,
		StartedAt: now,
		UpdatedAt: now,
		Steps: []StepState{
			{Name: "step-1", Status: StepDone, IdempotencyKey: "saga-sql-1:step-1:execute"},
		},
		Data: map[string]any{"key": "value"},
	}

	if err := store.Save(ctx, st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(ctx, "saga-sql-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID != "saga-sql-1" {
		t.Errorf("expected ID saga-sql-1, got %s", loaded.ID)
	}
	if loaded.Status != SagaRunning {
		t.Errorf("expected status Running, got %s", loaded.Status)
	}
	if len(loaded.Steps) != 1 || loaded.Steps[0].Status != StepDone {
		t.Errorf("unexpected steps: %+v", loaded.Steps)
	}
	val, ok := loaded.GetData("key")
	if !ok || val != "value" {
		t.Errorf("expected data key=value, got %v ok=%v", val, ok)
	}
}

func TestSQLStore_LoadNotFound(t *testing.T) {
	store := newSQLiteStore(t)
	_, err := store.Load(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent saga")
	}
}

func TestSQLStore_UpdateStepAndOutbox(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	st := &SagaState{
		ID:        "saga-sql-2",
		SagaName:  "test",
		Status:    SagaRunning,
		StartedAt: now,
		UpdatedAt: now,
		Steps:     []StepState{{Name: "step-1", Status: StepPending}},
	}
	_ = store.Save(ctx, st)

	executedAt := time.Now().UTC()
	updatedStep := StepState{
		Name:        "step-1",
		Status:      StepDone,
		Attempt:     1,
		ExecutedAt:  &executedAt,
	}
	outboxRecs := []OutboxRecord{
		{ID: "ob-sql-1", SagaID: "saga-sql-2", StepName: "step-1", EventType: "Created", Payload: []byte("{}"), CreatedAt: now},
	}

	if err := store.UpdateStepAndOutbox(ctx, "saga-sql-2", 0, updatedStep, outboxRecs); err != nil {
		t.Fatalf("UpdateStepAndOutbox: %v", err)
	}

	loaded, _ := store.Load(ctx, "saga-sql-2")
	if loaded.Steps[0].Status != StepDone {
		t.Errorf("expected step Done, got %s", loaded.Steps[0].Status)
	}

	recs, err := store.ListPendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("ListPendingOutbox: %v", err)
	}
	if len(recs) != 1 || recs[0].ID != "ob-sql-1" {
		t.Fatalf("unexpected outbox records: %+v", recs)
	}
}

func TestSQLStore_ListIncomplete(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = store.Save(ctx, &SagaState{ID: "s1", SagaName: "t", Status: SagaRunning, StartedAt: now, UpdatedAt: now, Steps: []StepState{}})
	_ = store.Save(ctx, &SagaState{ID: "s2", SagaName: "t", Status: SagaCompleted, StartedAt: now, UpdatedAt: now, Steps: []StepState{}})
	_ = store.Save(ctx, &SagaState{ID: "s3", SagaName: "t", Status: SagaCompensating, StartedAt: now, UpdatedAt: now, Steps: []StepState{}})

	incomplete, err := store.ListIncomplete(ctx)
	if err != nil {
		t.Fatalf("ListIncomplete: %v", err)
	}
	if len(incomplete) != 2 {
		t.Fatalf("expected 2 incomplete, got %d", len(incomplete))
	}
}

func TestSQLStore_ClaimSaga(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = store.Save(ctx, &SagaState{ID: "s1", SagaName: "t", Status: SagaRunning, StartedAt: now, UpdatedAt: now, Steps: []StepState{}})

	claimed1, err := store.ClaimSaga(ctx, "s1", "instance-1")
	if err != nil {
		t.Fatalf("ClaimSaga 1: %v", err)
	}
	if !claimed1 {
		t.Fatal("expected first claim to succeed")
	}

	claimed2, _ := store.ClaimSaga(ctx, "s1", "instance-2")
	if claimed2 {
		t.Fatal("expected second claim from different instance to fail")
	}

	_ = store.ClearClaim(ctx, "s1")

	claimed3, _ := store.ClaimSaga(ctx, "s1", "instance-2")
	if !claimed3 {
		t.Fatal("expected claim after clear to succeed")
	}
}

func TestSQLStore_OutboxLifecycle(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	rec := OutboxRecord{
		ID: "ob-1", SagaID: "s1", StepName: "step-1", EventType: "Created",
		Payload: []byte(`{"id":1}`), CreatedAt: now, Status: OutboxPending,
	}
	if err := store.AppendOutbox(ctx, rec); err != nil {
		t.Fatalf("AppendOutbox: %v", err)
	}

	recs, _ := store.ListPendingOutbox(ctx, 10)
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
		ID: "ob-2", SagaID: "s1", StepName: "step-2", EventType: "Updated",
		Payload: []byte(`{}`), CreatedAt: now, Status: OutboxPending,
	}
	_ = store.AppendOutbox(ctx, rec2)

	for i := 0; i < 3; i++ {
		_ = store.MarkFailed(ctx, "ob-2", "publish error", 3)
	}

	recs, _ = store.ListPendingOutbox(ctx, 10)
	if len(recs) != 0 {
		t.Fatalf("expected 0 pending after 3 failures, got %d", len(recs))
	}
}
