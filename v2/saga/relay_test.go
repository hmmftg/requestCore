package saga

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOutboxRelay_PublishesPendingRecords(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now()

	_ = store.AppendOutbox(context.Background(), OutboxRecord{
		ID: "ob-1", SagaID: "s1", StepName: "step-1", EventType: "Created",
		Payload: []byte("{}"), CreatedAt: now, Status: OutboxPending,
	})
	_ = store.AppendOutbox(context.Background(), OutboxRecord{
		ID: "ob-2", SagaID: "s1", StepName: "step-2", EventType: "Updated",
		Payload: []byte("{}"), CreatedAt: now, Status: OutboxPending,
	})

	var published []string
	var mu sync.Mutex
	broker := &mockBroker{
		publishFn: func(_ context.Context, rec OutboxRecord) error {
			mu.Lock()
			published = append(published, rec.ID)
			mu.Unlock()
			return nil
		},
	}

	relay := NewOutboxRelay(store, broker, WithRelayBatchSize(10))
	err := relay.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(published) != 2 {
		t.Fatalf("expected 2 published, got %d", len(published))
	}

	recs, _ := store.ListPendingOutbox(context.Background(), 10)
	if len(recs) != 0 {
		t.Fatalf("expected 0 pending after relay, got %d", len(recs))
	}
}

func TestOutboxRelay_BrokerFailureRetries(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now()

	_ = store.AppendOutbox(context.Background(), OutboxRecord{
		ID: "ob-1", SagaID: "s1", StepName: "step-1", EventType: "Created",
		Payload: []byte("{}"), CreatedAt: now, Status: OutboxPending,
	})

	var failCount atomic.Int32
	broker := &mockBroker{
		publishFn: func(_ context.Context, _ OutboxRecord) error {
			failCount.Add(1)
			return errors.New("broker unavailable")
		},
	}

	relay := NewOutboxRelay(store, broker, WithRelayBatchSize(10), WithRelayFailThreshold(3))

	_ = relay.Tick(context.Background())
	recs, _ := store.ListPendingOutbox(context.Background(), 10)
	if len(recs) != 1 {
		t.Fatalf("expected 1 pending after first failed tick, got %d", len(recs))
	}

	_ = relay.Tick(context.Background())
	recs, _ = store.ListPendingOutbox(context.Background(), 10)
	if len(recs) != 1 {
		t.Fatalf("expected 1 pending after second failed tick, got %d", len(recs))
	}

	_ = relay.Tick(context.Background())
	recs, _ = store.ListPendingOutbox(context.Background(), 10)
	if len(recs) != 0 {
		t.Fatalf("expected 0 pending after 3 failures (marked Failed), got %d", len(recs))
	}

	if failCount.Load() != 3 {
		t.Errorf("expected 3 publish attempts, got %d", failCount.Load())
	}
}

func TestOutboxRelay_NoRecords(t *testing.T) {
	store := NewMemoryStore()
	broker := &mockBroker{}
	relay := NewOutboxRelay(store, broker)

	err := relay.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick with no records: %v", err)
	}
}

func TestOutboxRelay_NopBrokerDefault(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now()

	_ = store.AppendOutbox(context.Background(), OutboxRecord{
		ID: "ob-1", SagaID: "s1", StepName: "step-1", EventType: "Created",
		Payload: []byte("{}"), CreatedAt: now, Status: OutboxPending,
	})

	relay := NewOutboxRelay(store, nil)

	err := relay.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick with NopBroker: %v", err)
	}

	recs, _ := store.ListPendingOutbox(context.Background(), 10)
	if len(recs) != 0 {
		t.Fatalf("expected 0 pending after NopBroker publish, got %d", len(recs))
	}
}

func TestOutboxRelay_ScheduledJob(t *testing.T) {
	store := NewMemoryStore()
	relay := NewOutboxRelay(store, NopBroker{})

	job := relay.ScheduledJob(5 * time.Second)
	if job.Name != "outbox-relay" {
		t.Errorf("expected job name outbox-relay, got %s", job.Name)
	}
	if job.Interval != 5*time.Second {
		t.Errorf("expected interval 5s, got %v", job.Interval)
	}
	if job.Handler == nil {
		t.Error("expected non-nil handler")
	}
}

type mockBroker struct {
	publishFn func(context.Context, OutboxRecord) error
}

func (m *mockBroker) Publish(ctx context.Context, rec OutboxRecord) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, rec)
	}
	return nil
}
