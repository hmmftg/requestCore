package libTracing

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingManager manages OpenTelemetry tracing
type TracingManager struct {
	config         *TracingConfig
	tracer         trace.Tracer
	tracerProvider *sdktrace.TracerProvider
	shutdown       func(context.Context) error
}

// DefaultTracingConfig returns a default tracing configuration
func DefaultTracingConfig() *TracingConfig {
	return &TracingConfig{
		ServiceName:    "requestCore",
		ServiceVersion: "1.0.0",
		Environment:    "development",
		Exporter:       "stdout",
		SamplingRatio:  1.0,
		Enabled:        true,
		Attributes: map[string]string{
			"service.name":    "requestCore",
			"service.version": "1.0.0",
		},
	}
}

// NewTracingManager creates a new tracing manager
func NewTracingManager(config *TracingConfig) (*TracingManager, error) {
	if config == nil {
		config = DefaultTracingConfig()
	}

	if !config.Enabled {
		return &TracingManager{
			config: config,
			tracer: trace.NewNoopTracerProvider().Tracer("noop"),
		}, nil
	}

	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(config.Environment),
		),
		resource.WithAttributes(func() []attribute.KeyValue {
			var attrs []attribute.KeyValue
			for k, v := range config.Attributes {
				attrs = append(attrs, attribute.String(k, v))
			}
			return attrs
		}()...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter
	var exporter sdktrace.SpanExporter
	switch config.Exporter {
	case "jaeger":
		exporter, err = jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.JaegerEndpoint)))
		if err != nil {
			return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
		}
	case "zipkin":
		exporter, err = zipkin.New(config.ZipkinEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to create Zipkin exporter: %w", err)
		}
	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	case "none":
		exporter = &noopExporter{}
	default:
		return nil, fmt.Errorf("unsupported exporter: %s", config.Exporter)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SamplingRatio)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &TracingManager{
		config:         config,
		tracer:         tp.Tracer(config.ServiceName),
		tracerProvider: tp,
		shutdown:       tp.Shutdown,
	}, nil
}

// GetTracer returns the tracer instance
func (tm *TracingManager) GetTracer() trace.Tracer {
	return tm.tracer
}

// StartSpan starts a new span
func (tm *TracingManager) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return tm.tracer.Start(ctx, name, opts...)
}

// StartSpanWithAttributes starts a new span with attributes
func (tm *TracingManager) StartSpanWithAttributes(ctx context.Context, name string, attrs map[string]string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	var spanOpts []trace.SpanStartOption
	spanOpts = append(spanOpts, opts...)

	for k, v := range attrs {
		spanOpts = append(spanOpts, trace.WithAttributes(attribute.String(k, v)))
	}

	return tm.tracer.Start(ctx, name, spanOpts...)
}

// AddSpanAttributes adds attributes to the current span
func (tm *TracingManager) AddSpanAttributes(ctx context.Context, attrs map[string]string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		for k, v := range attrs {
			span.SetAttributes(attribute.String(k, v))
		}
	}
}

// AddSpanEvent adds an event to the current span
func (tm *TracingManager) AddSpanEvent(ctx context.Context, name string, attrs map[string]string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.AddEvent(name, trace.WithAttributes(eventAttrs...))
	}
}

// RecordError records an error in the current span
func (tm *TracingManager) RecordError(ctx context.Context, err error, attrs map[string]string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.RecordError(err, trace.WithAttributes(eventAttrs...))
	}
}

// Shutdown gracefully shuts down the tracing manager
func (tm *TracingManager) Shutdown(ctx context.Context) error {
	if tm.shutdown != nil {
		return tm.shutdown(ctx)
	}
	return nil
}

// ExtractTraceContext extracts trace context from headers
func (tm *TracingManager) ExtractTraceContext(ctx context.Context, headers map[string]string) context.Context {
	carrier := &headerCarrier{headers: headers}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// InjectTraceContext injects trace context into headers
func (tm *TracingManager) InjectTraceContext(ctx context.Context, headers map[string]string) {
	carrier := &headerCarrier{headers: headers}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// noopExporter is a no-op span exporter
type noopExporter struct{}

func (e *noopExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	return nil
}

func (e *noopExporter) Shutdown(ctx context.Context) error {
	return nil
}

// headerCarrier implements the TextMapCarrier interface
type headerCarrier struct {
	headers map[string]string
}

func (c *headerCarrier) Get(key string) string {
	return c.headers[key]
}

func (c *headerCarrier) Set(key, value string) {
	c.headers[key] = value
}

func (c *headerCarrier) Keys() []string {
	var keys []string
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}

// TracingMiddleware creates middleware for HTTP tracing
func TracingMiddleware(tm *TracingManager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from headers
			ctx := tm.ExtractTraceContext(r.Context(), headersToMap(r.Header))

			// Start span
			spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
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
				"http.status_code": fmt.Sprintf("%d", wrapped.statusCode),
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

// headersToMap converts http.Header to map[string]string
func headersToMap(headers http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

// InitGlobalTracing initializes global tracing
func InitGlobalTracing(config *TracingConfig) error {
	tm, err := NewTracingManager(config)
	if err != nil {
		return err
	}
	globalTracingManager = tm
	return nil
}

// ShutdownGlobalTracing shuts down global tracing
func ShutdownGlobalTracing(ctx context.Context) error {
	if globalTracingManager != nil {
		return globalTracingManager.Shutdown(ctx)
	}
	return nil
}

// Helper functions for common tracing operations
func StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, trace.Span) {
	tm := GetGlobalTracingManager()
	return tm.StartSpanWithAttributes(ctx, name, attrs)
}

func AddSpanAttributes(ctx context.Context, attrs map[string]string) {
	tm := GetGlobalTracingManager()
	tm.AddSpanAttributes(ctx, attrs)
}

func AddSpanEvent(ctx context.Context, name string, attrs map[string]string) {
	tm := GetGlobalTracingManager()
	tm.AddSpanEvent(ctx, name, attrs)
}

func RecordError(ctx context.Context, err error, attrs map[string]string) {
	tm := GetGlobalTracingManager()
	tm.RecordError(ctx, err, attrs)
}

// Logging integration
func LogWithTrace(ctx context.Context, logger *slog.Logger, level slog.Level, msg string, attrs ...slog.Attr) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		// Add trace context to log attributes
		traceAttrs := []slog.Attr{
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		}
		attrs = append(traceAttrs, attrs...)
	}
	logger.LogAttrs(ctx, level, msg, attrs...)
}
