package libNetHttp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hmmftg/requestCore/libTracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware creates net/http middleware for OpenTelemetry tracing
func TracingMiddleware() func(http.Handler) http.Handler {
	// Use the official net/http instrumentation
	return otelhttp.NewMiddleware("requestCore")
}

// CustomTracingMiddleware creates a custom net/http middleware with more control
func CustomTracingMiddleware(tm *libTracing.TracingManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from headers
			headers := make(map[string]string)
			for k, v := range r.Header {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}

			ctx := tm.ExtractTraceContext(r.Context(), headers)

			// Start span
			spanName := r.Method + " " + r.URL.Path
			ctx, span := tm.StartSpanWithAttributes(ctx, spanName, map[string]string{
				"http.method":     r.Method,
				"http.url":        r.URL.String(),
				"http.user_agent": r.UserAgent(),
				"http.request_id": r.Header.Get("Request-Id"),
			})
			defer span.End()

			// Add request attributes
			tm.AddSpanAttributes(ctx, map[string]string{
				"http.scheme": r.URL.Scheme,
				"http.host":   r.Host,
				"http.target": r.URL.Path,
				"http.route":  r.URL.Path,
			})

			// Create response writer wrapper
			wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

			// Process request
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Add response attributes
			tm.AddSpanAttributes(ctx, map[string]string{
				"http.status_code": string(rune(wrapped.statusCode)),
			})

			// Set response status
			if wrapped.statusCode >= 400 {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", wrapped.statusCode))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// AddSpanAttribute adds an attribute to the current span
func AddSpanAttribute(ctx context.Context, key, value string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attribute.String(key, value))
	}
}

// AddSpanAttributes adds multiple attributes to the current span
func AddSpanAttributes(ctx context.Context, attrs map[string]string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		for k, v := range attrs {
			span.SetAttributes(attribute.String(k, v))
		}
	}
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs map[string]string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.AddEvent(name, trace.WithAttributes(eventAttrs...))
	}
}

// RecordSpanError records an error in the current span
func RecordSpanError(ctx context.Context, err error, attrs map[string]string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.RecordError(err, trace.WithAttributes(eventAttrs...))
	}
}

// GetSpanFromContext gets the span from context
func GetSpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}
