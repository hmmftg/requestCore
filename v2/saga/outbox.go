package saga

import (
	"context"
	"time"
)

// OutboxRecord represents an event to be reliably delivered to an
// external system via the outbox pattern. Records are appended
// atomically with step state updates and later published by an
// OutboxRelay.
type OutboxRecord struct {
	// ID is the unique identifier for this outbox record.
	ID string

	// SagaID is the saga that produced this event.
	SagaID string

	// StepName is the step that produced this event.
	StepName string

	// EventType is the domain event type (e.g. "OrderCreated").
	EventType string

	// Payload is the JSON-encoded event body.
	Payload []byte

	// CreatedAt is when the record was appended to the outbox.
	CreatedAt time.Time

	// PublishedAt is when the record was successfully published.
	// Nil if not yet published.
	PublishedAt *time.Time

	// Status is the current delivery status.
	Status OutboxStatus

	// FailCount is the number of consecutive publish failures.
	FailCount int

	// LastError holds the most recent publish error message.
	LastError string
}

// Broker publishes outbox events to an external system (e.g. Kafka,
// RabbitMQ, NATS). Implementations must be safe for concurrent use.
type Broker interface {
	// Publish delivers the record to the external system. Returning
	// an error causes the relay to retry on the next tick.
	Publish(ctx context.Context, rec OutboxRecord) error
}

// NopBroker is a no-op Broker that discards all events. It is the
// default when no broker is configured.
type NopBroker struct{}

// Publish discards the record and returns nil.
func (NopBroker) Publish(context.Context, OutboxRecord) error { return nil }
