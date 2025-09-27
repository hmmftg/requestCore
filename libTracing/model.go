package libTracing

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SpanContext holds span context information
type SpanContext struct {
	TraceID string `json:"trace_id"`
	SpanID  string `json:"span_id"`
	Sampled bool   `json:"sampled"`
}

// SpanInfo holds span information for logging and debugging
type SpanInfo struct {
	Name       string            `json:"name"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	Duration   time.Duration     `json:"duration"`
	Attributes map[string]string `json:"attributes"`
	Events     []SpanEvent       `json:"events"`
	Status     SpanStatus        `json:"status"`
}

// SpanEvent represents an event within a span
type SpanEvent struct {
	Name       string            `json:"name"`
	Timestamp  time.Time         `json:"timestamp"`
	Attributes map[string]string `json:"attributes"`
}

// SpanStatus represents the status of a span
type SpanStatus struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// TracingContext extends context.Context with tracing capabilities
type TracingContext struct {
	context.Context
	span trace.Span
}

// NewTracingContext creates a new tracing context
func NewTracingContext(ctx context.Context, span trace.Span) *TracingContext {
	return &TracingContext{
		Context: ctx,
		span:    span,
	}
}

// GetSpan returns the current span
func (tc *TracingContext) GetSpan() trace.Span {
	return tc.span
}

// GetSpanContext returns span context information
func (tc *TracingContext) GetSpanContext() SpanContext {
	if tc.span == nil {
		return SpanContext{}
	}

	spanCtx := tc.span.SpanContext()
	return SpanContext{
		TraceID: spanCtx.TraceID().String(),
		SpanID:  spanCtx.SpanID().String(),
		Sampled: spanCtx.IsSampled(),
	}
}

// AddAttribute adds an attribute to the span
func (tc *TracingContext) AddAttribute(key, value string) {
	if tc.span != nil && tc.span.IsRecording() {
		tc.span.SetAttributes(attribute.String(key, value))
	}
}

// AddAttributes adds multiple attributes to the span
func (tc *TracingContext) AddAttributes(attrs map[string]string) {
	if tc.span != nil && tc.span.IsRecording() {
		for k, v := range attrs {
			tc.span.SetAttributes(attribute.String(k, v))
		}
	}
}

// AddEvent adds an event to the span
func (tc *TracingContext) AddEvent(name string, attrs map[string]string) {
	if tc.span != nil && tc.span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		tc.span.AddEvent(name, trace.WithAttributes(eventAttrs...))
	}
}

// RecordError records an error in the span
func (tc *TracingContext) RecordError(err error, attrs map[string]string) {
	if tc.span != nil && tc.span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		tc.span.RecordError(err, trace.WithAttributes(eventAttrs...))
	}
}

// SetStatus sets the status of the span
func (tc *TracingContext) SetStatus(code codes.Code, description string) {
	if tc.span != nil && tc.span.IsRecording() {
		tc.span.SetStatus(code, description)
	}
}

// End ends the span
func (tc *TracingContext) End() {
	if tc.span != nil {
		tc.span.End()
	}
}

// TracingAttributes defines common tracing attributes
type TracingAttributes struct {
	// HTTP attributes
	HTTPMethod     string `json:"http.method"`
	HTTPURL        string `json:"http.url"`
	HTTPStatusCode string `json:"http.status_code"`
	HTTPUserAgent  string `json:"http.user_agent"`

	// Request attributes
	RequestID string `json:"request.id"`
	UserID    string `json:"user.id"`
	ProgramID string `json:"program.id"`
	ModuleID  string `json:"module.id"`
	MethodID  string `json:"method.id"`

	// Database attributes
	DBOperation string `json:"db.operation"`
	DBStatement string `json:"db.statement"`
	DBTable     string `json:"db.table"`

	// Custom attributes
	CustomAttributes map[string]string `json:"custom_attributes"`
}

// ToMap converts TracingAttributes to map[string]string
func (ta *TracingAttributes) ToMap() map[string]string {
	attrs := make(map[string]string)

	if ta.HTTPMethod != "" {
		attrs["http.method"] = ta.HTTPMethod
	}
	if ta.HTTPURL != "" {
		attrs["http.url"] = ta.HTTPURL
	}
	if ta.HTTPStatusCode != "" {
		attrs["http.status_code"] = ta.HTTPStatusCode
	}
	if ta.HTTPUserAgent != "" {
		attrs["http.user_agent"] = ta.HTTPUserAgent
	}
	if ta.RequestID != "" {
		attrs["request.id"] = ta.RequestID
	}
	if ta.UserID != "" {
		attrs["user.id"] = ta.UserID
	}
	if ta.ProgramID != "" {
		attrs["program.id"] = ta.ProgramID
	}
	if ta.ModuleID != "" {
		attrs["module.id"] = ta.ModuleID
	}
	if ta.MethodID != "" {
		attrs["method.id"] = ta.MethodID
	}
	if ta.DBOperation != "" {
		attrs["db.operation"] = ta.DBOperation
	}
	if ta.DBStatement != "" {
		attrs["db.statement"] = ta.DBStatement
	}
	if ta.DBTable != "" {
		attrs["db.table"] = ta.DBTable
	}

	// Add custom attributes
	for k, v := range ta.CustomAttributes {
		attrs[k] = v
	}

	return attrs
}
