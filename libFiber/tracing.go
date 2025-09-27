package libFiber

import (
	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/hmmftg/requestCore/libTracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware creates Fiber middleware for OpenTelemetry tracing using official otelfiber
func TracingMiddleware() fiber.Handler {
	// Use the official Fiber instrumentation
	return otelfiber.Middleware()
}

// CustomTracingMiddleware creates a custom Fiber middleware with more control
func CustomTracingMiddleware(tm *libTracing.TracingManager) fiber.Handler {
	return otelfiber.Middleware()
}

// AddSpanAttribute adds an attribute to the current span in Fiber context
func AddSpanAttribute(c *fiber.Ctx, key, value string) {
	span := trace.SpanFromContext(c.UserContext())
	if span.IsRecording() {
		span.SetAttributes(attribute.String(key, value))
	}
}

// AddSpanAttributes adds multiple attributes to the current span
func AddSpanAttributes(c *fiber.Ctx, attrs map[string]string) {
	span := trace.SpanFromContext(c.UserContext())
	if span.IsRecording() {
		for k, v := range attrs {
			span.SetAttributes(attribute.String(k, v))
		}
	}
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(c *fiber.Ctx, name string, attrs map[string]string) {
	span := trace.SpanFromContext(c.UserContext())
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.AddEvent(name, trace.WithAttributes(eventAttrs...))
	}
}

// RecordSpanError records an error in the current span
func RecordSpanError(c *fiber.Ctx, err error, attrs map[string]string) {
	span := trace.SpanFromContext(c.UserContext())
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.RecordError(err, trace.WithAttributes(eventAttrs...))
	}
}

// GetSpanFromContext gets the span from Fiber context
func GetSpanFromContext(c *fiber.Ctx) trace.Span {
	return trace.SpanFromContext(c.UserContext())
}
