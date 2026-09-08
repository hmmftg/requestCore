package httpsemantics

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// ExtractW3CTraceContext extracts W3C Trace Context headers (traceparent,
// tracestate) from the given HTTP request and returns a new context with
// the trace context embedded. This is the inbound extraction point for
// W3C trace propagation.
//
// This helper uses the globally configured OpenTelemetry propagator
// (otel.GetTextMapPropagator()), which is typically a composite of
// propagation.TraceContext{} and propagation.Baggage{} as configured
// by libTracing.
//
// Usage in framework adapters:
//
//	ctx := httpsemantics.ExtractW3CTraceContext(r.Context(), r.Header)
//	// Pass ctx to request.NewContext or use it directly
func ExtractW3CTraceContext(parent context.Context, h http.Header) context.Context {
	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		return parent
	}
	return propagator.Extract(parent, propagation.HeaderCarrier(h))
}

// InjectW3CTraceContext injects the W3C Trace Context headers from the
// given context into the provided HTTP headers. This is the outbound
// injection point for W3C trace propagation when making downstream
// API calls.
//
// Usage:
//
//	httpsemantics.InjectW3CTraceContext(ctx, req.Header)
//	// Then send req via http.Client
func InjectW3CTraceContext(ctx context.Context, h http.Header) {
	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		return
	}
	propagator.Inject(ctx, propagation.HeaderCarrier(h))
}

// ExtractW3CTraceContextFromMap extracts W3C Trace Context headers from
// a map[string]string (e.g. framework parser header map) and returns a
// new context with the trace context embedded.
func ExtractW3CTraceContextFromMap(parent context.Context, headers map[string]string) context.Context {
	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		return parent
	}
	return propagator.Extract(parent, propagation.MapCarrier(headers))
}

// InjectW3CTraceContextToMap injects W3C Trace Context headers from the
// given context into a map[string]string.
func InjectW3CTraceContextToMap(ctx context.Context, headers map[string]string) {
	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		return
	}
	propagator.Inject(ctx, propagation.MapCarrier(headers))
}
