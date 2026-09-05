// Package saga implements the Saga distributed-transaction pattern
// with step compensation, durable state persistence, crash recovery,
// outbox relay, and step-level idempotency.
//
// # Overview
//
// A Saga is a sequence of local transactions (Steps) where each step
// has an Execute function (forward action) and a Compensate function
// (undo action). If any step fails, the orchestrator compensates
// completed steps in reverse order.
//
// # Telemetry
//
// All saga lifecycle events are recorded through telemetry.Sink,
// consistent with the v2 kernel's observability path. Per-step events
// use the operation key "saga-<id>-<step>-req" (success) or
// "saga-<id>-<step>-req-failed" (failure), matching the v1 AddLog
// convention so Splunk dashboards remain consistent.
//
// # Persistence and Crash Recovery
//
// When a SagaStore is configured, the orchestrator persists saga state
// before and after each step. If the process crashes, ResumeAll loads
// incomplete sagas and resumes them from their last persisted state.
// Concurrent resume across multiple instances is protected by
// ClaimSaga.
//
// # Outbox Pattern
//
// Steps can append OutboxRecords during execution. The orchestrator
// flushes them atomically with the step state update via
// SagaStore.UpdateStepAndOutbox. An OutboxRelay polls the outbox and
// publishes pending records to a Broker on a configurable interval.
//
// # Idempotency
//
// Each step execution and compensation has a unique IdempotencyKey
// (format: "<sagaID>:<stepName>:<phase>"). On resume, steps that are
// already Done or Compensated are skipped, preventing duplicate
// side-effects.
//
// # Usage
//
//	orch := saga.NewOrchestrator(
//	    saga.WithStore(store),
//	    saga.WithSink(sink),
//	    saga.WithTimeout(30 * time.Second),
//	)
//
//	s := &saga.Saga{
//	    ID:   "checkout-123",
//	    Name: "checkout",
//	    Steps: []saga.Step{
//	        {Name: "reserve-inventory", Execute: reserveFn, Compensate: releaseFn},
//	        {Name: "charge-card", Execute: chargeFn, Compensate: refundFn},
//	        {Name: "ship-order", Execute: shipFn},
//	    },
//	}
//
//	if err := orch.Execute(ctx, s); err != nil {
//	    log.Printf("saga failed: %v", err)
//	}
//
// # Architecture
//
// The saga package imports only telemetry, workers, and the Go
// standard library. It does not import app, routing, endpoint,
// response, or any v1 package.
package saga
