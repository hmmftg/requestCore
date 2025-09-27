package libGin

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libTracing"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware creates Gin middleware for OpenTelemetry tracing
func TracingMiddleware() gin.HandlerFunc {
	// Use the official Gin instrumentation
	return otelgin.Middleware("requestCore")
}

// CustomTracingMiddleware creates a custom Gin middleware with more control
func CustomTracingMiddleware(tm *libTracing.TracingManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract trace context from headers
		headers := make(map[string]string)
		for k, v := range c.Request.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}

		ctx := tm.ExtractTraceContext(c.Request.Context(), headers)

		// Start span
		spanName := c.Request.Method + " " + c.Request.URL.Path
		ctx, span := tm.StartSpanWithAttributes(ctx, spanName, map[string]string{
			"http.method":     c.Request.Method,
			"http.url":        c.Request.URL.String(),
			"http.user_agent": c.Request.UserAgent(),
			"http.request_id": c.GetHeader("Request-Id"),
		})
		defer span.End()

		// Add request attributes
		tm.AddSpanAttributes(ctx, map[string]string{
			"http.scheme": c.Request.URL.Scheme,
			"http.host":   c.Request.Host,
			"http.target": c.Request.URL.Path,
			"http.route":  c.Request.URL.Path,
		})

		// Set span in Gin context
		c.Set("span", span)

		// Process request
		c.Next()

		// Add response attributes
		tm.AddSpanAttributes(ctx, map[string]string{
			"http.status_code": string(rune(c.Writer.Status())),
		})

		// Set response status
		if c.Writer.Status() >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", c.Writer.Status()))
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}
}

// AddSpanAttribute adds an attribute to the current span in Gin context
func AddSpanAttribute(c *gin.Context, key, value string) {
	if span, exists := c.Get("span"); exists {
		if s, ok := span.(trace.Span); ok && s.IsRecording() {
			s.SetAttributes(attribute.String(key, value))
		}
	}
}

// AddSpanAttributes adds multiple attributes to the current span
func AddSpanAttributes(c *gin.Context, attrs map[string]string) {
	if span, exists := c.Get("span"); exists {
		if s, ok := span.(trace.Span); ok && s.IsRecording() {
			for k, v := range attrs {
				s.SetAttributes(attribute.String(k, v))
			}
		}
	}
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(c *gin.Context, name string, attrs map[string]string) {
	if span, exists := c.Get("span"); exists {
		if s, ok := span.(trace.Span); ok && s.IsRecording() {
			var eventAttrs []attribute.KeyValue
			for k, v := range attrs {
				eventAttrs = append(eventAttrs, attribute.String(k, v))
			}
			s.AddEvent(name, trace.WithAttributes(eventAttrs...))
		}
	}
}

// RecordSpanError records an error in the current span
func RecordSpanError(c *gin.Context, err error, attrs map[string]string) {
	if span, exists := c.Get("span"); exists {
		if s, ok := span.(trace.Span); ok && s.IsRecording() {
			var eventAttrs []attribute.KeyValue
			for k, v := range attrs {
				eventAttrs = append(eventAttrs, attribute.String(k, v))
			}
			s.RecordError(err, trace.WithAttributes(eventAttrs...))
		}
	}
}

// GetSpanFromContext gets the span from Gin context
func GetSpanFromContext(c *gin.Context) trace.Span {
	if span, exists := c.Get("span"); exists {
		if s, ok := span.(trace.Span); ok {
			return s
		}
	}
	return nil
}
