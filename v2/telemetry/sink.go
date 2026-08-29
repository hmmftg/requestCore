// Package telemetry provides framework-neutral event recording for the
// redesigned v2 kernel.
//
// A Sink receives typed lifecycle and business events. The public
// SlogSink records events via slog WITHOUT including request or
// response bodies by default. Enterprise AddLog compatibility is
// handled separately by compat/v1 in later tranches; this package does
// not replace webFramework.AddLog.
//
// telemetry imports only the Go standard library. It does not import
// request, response, handlers, app, or any v1 package.
package telemetry

import (
	"log/slog"
	"time"
)

// EventType classifies a telemetry event.
type EventType int

const (
	// EventStart is emitted at the beginning of request processing.
	EventStart EventType = iota

	// EventSuccess is emitted when the request completes successfully.
	EventSuccess

	// EventFailure is emitted when the request fails.
	EventFailure

	// EventBusiness is emitted for custom business events during
	// request processing.
	EventBusiness
)

// String returns a human-readable name for the event type.
func (e EventType) String() string {
	switch e {
	case EventStart:
		return "start"
	case EventSuccess:
		return "success"
	case EventFailure:
		return "failure"
	case EventBusiness:
		return "business"
	default:
		return "unknown"
	}
}

// Level returns the slog.Level for the event type. Success and start
// are Info; failure is Error; business is Info.
func (e EventType) Level() slog.Level {
	switch e {
	case EventFailure:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Event represents a single telemetry event in the request lifecycle.
// Events are framework-neutral and do not include request or response
// bodies by default.
type Event struct {
	// Type is the event classification.
	Type EventType

	// Operation is the operation ID from the operation registry.
	Operation string

	// Method is the HTTP method.
	Method string

	// RoutePattern is the registered route pattern.
	RoutePattern string

	// Status is the HTTP response status code.
	Status int

	// Duration is the elapsed time for the request.
	Duration time.Duration

	// RequestID is the request identifier.
	RequestID string

	// TraceID is the trace identifier.
	TraceID string

	// Err is the error for failure events, or nil for success events.
	Err error

	// Attrs are additional safe attributes (never request/response bodies).
	Attrs []slog.Attr
}

// Sink receives telemetry events. Implementations must be safe for
// concurrent use.
type Sink interface {
	// Record processes a single telemetry event.
	Record(event Event)
}

// NopSink is a Sink that discards all events. Useful for testing.
type NopSink struct{}

// Record discards the event.
func (NopSink) Record(Event) {}
