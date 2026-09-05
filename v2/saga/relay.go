package saga

import (
	"context"
	"log/slog"
	"time"

	"github.com/hmmftg/requestCore/v2/telemetry"
	"github.com/hmmftg/requestCore/v2/workers"
)

// OutboxRelay polls the outbox table and publishes pending events to
// a Broker. It is designed to be registered as a workers.ScheduledJob
// on the existing workers.Scheduler.
type OutboxRelay struct {
	store         OutboxStore
	broker        Broker
	batchSize     int
	failThreshold int
	sink          telemetry.Sink
	clock         func() time.Time
}

// RelayOption configures an OutboxRelay at construction time.
type RelayOption func(*OutboxRelay)

// WithRelayBatchSize sets the number of records to process per tick.
// Default: 100.
func WithRelayBatchSize(n int) RelayOption {
	return func(r *OutboxRelay) { r.batchSize = n }
}

// WithRelayFailThreshold sets the number of consecutive failures before
// a record is marked as Failed. Default: 5.
func WithRelayFailThreshold(n int) RelayOption {
	return func(r *OutboxRelay) { r.failThreshold = n }
}

// WithRelaySink sets the telemetry sink for relay events.
func WithRelaySink(s telemetry.Sink) RelayOption {
	return func(r *OutboxRelay) { r.sink = s }
}

// NewOutboxRelay creates a new OutboxRelay with the given store and
// broker.
func NewOutboxRelay(store OutboxStore, broker Broker, opts ...RelayOption) *OutboxRelay {
	if broker == nil {
		broker = NopBroker{}
	}
	r := &OutboxRelay{
		store:         store,
		broker:        broker,
		batchSize:     100,
		failThreshold: 5,
		sink:          telemetry.NopSink{},
		clock:         time.Now,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Tick processes one batch of pending outbox records. This is the
// handler function for the ScheduledJob.
func (r *OutboxRelay) Tick(ctx context.Context) error {
	records, err := r.store.ListPendingOutbox(ctx, r.batchSize)
	if err != nil {
		r.recordEvent(telemetry.EventFailure, "outbox-relay-req", 0, err)
		return err
	}

	if len(records) == 0 {
		return nil
	}

	var lastErr error
	for _, rec := range records {
		start := r.clock()
		err := r.broker.Publish(ctx, rec)
		elapsed := r.clock().Sub(start)

		if err != nil {
			_ = r.store.MarkFailed(ctx, rec.ID, err.Error(), r.failThreshold)
			r.recordEvent(telemetry.EventFailure, "outbox-relay-req-failed", elapsed, err)
			lastErr = err
			continue
		}

		_ = r.store.MarkPublished(ctx, rec.ID)
		r.recordEvent(telemetry.EventSuccess, "outbox-relay-req", elapsed, nil)
	}

	return lastErr
}

// ScheduledJob returns a workers.ScheduledJob that runs the relay at
// the given interval. Register it on a workers.Scheduler.
func (r *OutboxRelay) ScheduledJob(interval time.Duration) workers.ScheduledJob {
	return workers.ScheduledJob{
		Name:     "outbox-relay",
		Handler:  r.jobHandler,
		Interval: interval,
		Options: workers.JobOptions{
			MaxAttempts: 1,
		},
	}
}

// jobHandler adapts Tick to the workers.JobHandler signature.
func (r *OutboxRelay) jobHandler(jctx *workers.JobContext) error {
	return r.Tick(jctx.Context)
}

// recordEvent emits a telemetry event for the relay.
func (r *OutboxRelay) recordEvent(eventType telemetry.EventType, operation string, duration time.Duration, err error) {
	if r.sink == nil {
		return
	}
	r.sink.Record(telemetry.Event{
		Type:      eventType,
		Operation: operation,
		Duration:  duration,
		Err:       err,
		Attrs: []slog.Attr{
			slog.String("component", "outbox-relay"),
		},
	})
}
