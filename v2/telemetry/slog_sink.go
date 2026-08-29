package telemetry

import (
	"context"
	"log/slog"
)

// SlogSink is a public telemetry Sink that records events via slog.
// It does NOT include request or response bodies by default. It records
// method, route pattern, operation, status, duration, request ID,
// trace ID, cache/idempotency outcome, and safe attributes.
//
// A nil logger defaults to slog.Default().
type SlogSink struct {
	logger *slog.Logger
}

// NewSlogSink creates a SlogSink wrapping the given logger. If logger
// is nil, slog.Default() is used.
func NewSlogSink(logger *slog.Logger) *SlogSink {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogSink{logger: logger}
}

// Record processes a telemetry event by emitting it via slog. The event
// is logged at Info level for start/success/business events and Error
// level for failure events. Request and response bodies are never
// included in the log output.
func (s *SlogSink) Record(event Event) {
	attrs := buildAttrs(event)
	s.logger.LogAttrs(context.Background(), event.Type.Level(), event.Type.String(), attrs...)
}

// buildAttrs constructs the slog attributes for an event. Bodies are
// never included.
func buildAttrs(event Event) []slog.Attr {
	attrs := make([]slog.Attr, 0, 8+len(event.Attrs))
	if event.Operation != "" {
		attrs = append(attrs, slog.String("operation", event.Operation))
	}
	if event.Method != "" {
		attrs = append(attrs, slog.String("method", event.Method))
	}
	if event.RoutePattern != "" {
		attrs = append(attrs, slog.String("route", event.RoutePattern))
	}
	if event.Status != 0 {
		attrs = append(attrs, slog.Int("status", event.Status))
	}
	if event.Duration != 0 {
		attrs = append(attrs, slog.String("duration", event.Duration.String()))
	}
	if event.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", event.RequestID))
	}
	if event.TraceID != "" {
		attrs = append(attrs, slog.String("trace_id", event.TraceID))
	}
	if event.Err != nil {
		attrs = append(attrs, slog.String("error", event.Err.Error()))
	}
	attrs = append(attrs, event.Attrs...)
	return attrs
}
