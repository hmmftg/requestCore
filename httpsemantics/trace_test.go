package httpsemantics

import (
	"context"
	"net/http"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestInjectExtractW3CTraceContext_RoundTrip(t *testing.T) {
	// Set up the W3C TraceContext propagator
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create a real tracer with a valid span context
	tp := trace.NewTracerProvider()
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(trace.NewTracerProvider()) // restore noop

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	h := http.Header{}
	InjectW3CTraceContext(ctx, h)

	// Verify traceparent header was injected
	tpHeader := h.Get("traceparent")
	if tpHeader == "" {
		t.Fatal("traceparent header not injected")
	}

	// Now extract it back
	extractedCtx := ExtractW3CTraceContext(context.Background(), h)
	extractedSpan := oteltrace.SpanFromContext(extractedCtx)
	if !extractedSpan.SpanContext().IsValid() {
		t.Fatal("extracted span context is not valid")
	}

	// The extracted trace ID should match the original
	originalTraceID := span.SpanContext().TraceID()
	extractedTraceID := extractedSpan.SpanContext().TraceID()
	if originalTraceID != extractedTraceID {
		t.Errorf("trace ID mismatch: original=%s extracted=%s", originalTraceID, extractedTraceID)
	}
}

func TestExtractW3CTraceContext_NoPropagator(t *testing.T) {
	// Save and restore the global propagator
	saved := otel.GetTextMapPropagator()
	defer otel.SetTextMapPropagator(saved)

	// Set a no-op propagator (empty composite)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())

	h := http.Header{}
	h.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")

	ctx := ExtractW3CTraceContext(context.Background(), h)
	// With a no-op propagator, the context should not have a valid span
	span := oteltrace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		t.Error("expected invalid span context with no-op propagator")
	}
}

func TestInjectW3CTraceContextToMap_RoundTrip(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tp := trace.NewTracerProvider()
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(trace.NewTracerProvider()) // restore noop

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	headers := make(map[string]string)
	InjectW3CTraceContextToMap(ctx, headers)

	if headers["traceparent"] == "" {
		t.Fatal("traceparent not injected to map")
	}

	extractedCtx := ExtractW3CTraceContextFromMap(context.Background(), headers)
	extractedSpan := oteltrace.SpanFromContext(extractedCtx)
	if !extractedSpan.SpanContext().IsValid() {
		t.Fatal("extracted span context is not valid")
	}

	originalTraceID := span.SpanContext().TraceID()
	extractedTraceID := extractedSpan.SpanContext().TraceID()
	if originalTraceID != extractedTraceID {
		t.Errorf("trace ID mismatch: original=%s extracted=%s", originalTraceID, extractedTraceID)
	}
}
