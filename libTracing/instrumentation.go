package libTracing

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hmmftg/requestCore/webFramework"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracableArg constraint for types that have RequestParser (like HandlerRequest)
type TracableArg interface {
	GetParser() webFramework.RequestParser
}

// getCallerInfo extracts function name and file info from the call stack
// skip is the number of stack frames to skip (0 = caller of getCallerInfo)
func getCallerInfo(skip int) (funcName, spanName string) {
	pc, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown", "unknown"
	}

	// Get function name
	fn := runtime.FuncForPC(pc)
	if fn != nil {
		fullName := fn.Name()
		// Extract just the function name (last part after last dot)
		parts := strings.Split(fullName, ".")
		funcName = parts[len(parts)-1]

		// Generate span name from function name (convert CamelCase to lowercase with dots)
		spanName = toSnakeCase(funcName)
	}

	// Add file info for debugging
	_, fileName := filepath.Split(file)
	_ = fileName
	_ = line

	return funcName, spanName
}

// toSnakeCase converts CamelCase to snake_case for span names
func toSnakeCase(s string) string {
	if s == "" {
		return "unknown"
	}

	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// isTracingEnabled checks if tracing is enabled for the given context
func isTracingEnabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	span := trace.SpanFromContext(ctx)
	return span != nil && span.IsRecording()
}

// traceWithSpan is the internal helper that handles span creation and execution
func traceWithSpan(ctx context.Context, spanName string, fn func(context.Context) (context.Context, error)) (context.Context, error) {
	// Fast path: if tracing is not enabled, execute function without tracing
	if !isTracingEnabled(ctx) {
		newCtx, err := fn(ctx)
		return newCtx, err
	}

	// Get tracing manager
	tm := GetGlobalTracingManager()
	if tm == nil {
		// Tracing manager not available, execute function normally
		newCtx, err := fn(ctx)
		return newCtx, err
	}

	// Start span with auto-generated attributes
	attrs := map[string]string{
		"function.name": spanName,
	}

	spanCtx, span := tm.StartSpanWithAttributes(ctx, spanName, attrs)
	if span == nil {
		// Span creation failed, execute function normally (zero-error design)
		newCtx, err := fn(ctx)
		return newCtx, err
	}

	// Measure execution time
	start := time.Now()

	// Execute function with span context
	newCtx, err := fn(spanCtx)

	// Record duration
	duration := time.Since(start)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("function.duration_ms", duration.Milliseconds()),
			attribute.Int64("function.duration_ns", duration.Nanoseconds()),
		)
	}

	// Record error if any (zero-error: handle internally, don't break execution)
	if err != nil {
		// Record error in span
		if span.IsRecording() {
			tm.RecordError(spanCtx, err, map[string]string{
				"error.type": "function_error",
			})
			span.SetStatus(codes.Error, err.Error())
		}
	} else {
		// Set success status
		if span.IsRecording() {
			span.SetStatus(codes.Ok, "")
		}
	}

	// End span
	span.End()

	return newCtx, err
}

// TraceFunc traces a function that returns (T, error) - Normal mode (automatic parser extraction)
// Usage: result, err := TraceFunc(handler.Handler, trx)
// Automatically extracts context from arg's RequestParser and updates it
func TraceFunc[T any, Arg TracableArg](fn func(Arg) (T, error), arg Arg) (T, error) {
	parser := arg.GetParser()
	return traceFuncWithParser(parser, fn, arg)
}

// TraceFuncWithParser traces a function that returns (T, error) - Manual mode (explicit parser)
// Usage: result, err := TraceFuncWithParser(parser, myFunction, myArg)
// Uses provided parser to extract and update context
func TraceFuncWithParser[T any, Arg any](parser webFramework.RequestParser, fn func(Arg) (T, error), arg Arg) (T, error) {
	return traceFuncWithParser(parser, fn, arg)
}

// traceFuncWithParser is the internal implementation shared by both modes
func traceFuncWithParser[T any, Arg any](parser webFramework.RequestParser, fn func(Arg) (T, error), arg Arg) (T, error) {
	// Get context from parser
	ctx := parser.GetContext()
	
	// Auto-detect span name from caller
	_, spanName := getCallerInfo(2)
	if spanName == "" || spanName == "unknown" {
		spanName = "function"
	}

	// Fast path: if tracing is not enabled, execute function without tracing
	if !isTracingEnabled(ctx) {
		return fn(arg)
	}

	// Get tracing manager
	tm := GetGlobalTracingManager()
	if tm == nil {
		// Tracing manager not available, execute function normally
		return fn(arg)
	}

	// Start span
	attrs := map[string]string{
		"function.name": spanName,
	}
	
	spanCtx, span := tm.StartSpanWithAttributes(ctx, spanName, attrs)
	if span == nil {
		// Span creation failed, execute function normally (zero-error design)
		return fn(arg)
	}

	// Measure execution time
	start := time.Now()

	// Execute function
	result, err := fn(arg)

	// Record duration
	duration := time.Since(start)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("function.duration_ms", duration.Milliseconds()),
			attribute.Int64("function.duration_ns", duration.Nanoseconds()),
		)
	}

	// Record error if any (zero-error: handle internally)
	if err != nil {
		if span.IsRecording() {
			tm.RecordError(spanCtx, err, map[string]string{
				"error.type": "function_error",
			})
			span.SetStatus(codes.Error, err.Error())
		}
	} else {
		if span.IsRecording() {
			span.SetStatus(codes.Ok, "")
		}
	}

	// End span
	span.End()

	// Update context in parser
	parser.SetContext(spanCtx)

	return result, err
}

// TraceError traces a function that returns only error - Normal mode (automatic parser extraction)
// Usage: err := TraceError(handler.Initializer, trx)
// Automatically extracts context from arg's RequestParser and updates it
func TraceError[Arg TracableArg](fn func(Arg) error, arg Arg) error {
	parser := arg.GetParser()
	return traceErrorWithParser(parser, fn, arg)
}

// TraceErrorWithParser traces a function that returns only error - Manual mode (explicit parser)
// Usage: err := TraceErrorWithParser(parser, myFunction, myArg)
// Uses provided parser to extract and update context
func TraceErrorWithParser[Arg any](parser webFramework.RequestParser, fn func(Arg) error, arg Arg) error {
	return traceErrorWithParser(parser, fn, arg)
}

// traceErrorWithParser is the internal implementation shared by both modes
func traceErrorWithParser[Arg any](parser webFramework.RequestParser, fn func(Arg) error, arg Arg) error {
	// Get context from parser
	ctx := parser.GetContext()
	
	// Auto-detect span name from caller
	_, spanName := getCallerInfo(2)
	if spanName == "" || spanName == "unknown" {
		spanName = "function"
	}

	// Fast path: if tracing is not enabled, execute function without tracing
	if !isTracingEnabled(ctx) {
		return fn(arg)
	}

	// Get tracing manager
	tm := GetGlobalTracingManager()
	if tm == nil {
		// Tracing manager not available, execute function normally
		return fn(arg)
	}

	// Start span
	attrs := map[string]string{
		"function.name": spanName,
	}
	
	spanCtx, span := tm.StartSpanWithAttributes(ctx, spanName, attrs)
	if span == nil {
		// Span creation failed, execute function normally (zero-error design)
		return fn(arg)
	}

	// Measure execution time
	start := time.Now()

	// Execute function
	err := fn(arg)

	// Record duration
	duration := time.Since(start)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("function.duration_ms", duration.Milliseconds()),
			attribute.Int64("function.duration_ns", duration.Nanoseconds()),
		)
	}

	// Record error if any (zero-error: handle internally)
	if err != nil {
		if span.IsRecording() {
			tm.RecordError(spanCtx, err, map[string]string{
				"error.type": "function_error",
			})
			span.SetStatus(codes.Error, err.Error())
		}
	} else {
		if span.IsRecording() {
			span.SetStatus(codes.Ok, "")
		}
	}

	// End span
	span.End()

	// Update context in parser
	parser.SetContext(spanCtx)

	return err
}

// TraceVoid traces a function with no return value - Normal mode (automatic parser extraction)
// Usage: TraceVoid(handler.Finalizer, trx)
// Automatically extracts context from arg's RequestParser and updates it
func TraceVoid[Arg TracableArg](fn func(Arg), arg Arg) {
	parser := arg.GetParser()
	traceVoidWithParser(parser, fn, arg)
}

// TraceVoidWithParser traces a function with no return value - Manual mode (explicit parser)
// Usage: TraceVoidWithParser(parser, myFunction, myArg)
// Uses provided parser to extract and update context
func TraceVoidWithParser[Arg any](parser webFramework.RequestParser, fn func(Arg), arg Arg) {
	traceVoidWithParser(parser, fn, arg)
}

// traceVoidWithParser is the internal implementation shared by both modes
func traceVoidWithParser[Arg any](parser webFramework.RequestParser, fn func(Arg), arg Arg) {
	// Get context from parser
	ctx := parser.GetContext()
	
	// Auto-detect span name from caller
	_, spanName := getCallerInfo(2)
	if spanName == "" || spanName == "unknown" {
		spanName = "function"
	}

	// Fast path: if tracing is not enabled, execute function without tracing
	if !isTracingEnabled(ctx) {
		fn(arg)
		return
	}

	// Get tracing manager
	tm := GetGlobalTracingManager()
	if tm == nil {
		// Tracing manager not available, execute function normally
		fn(arg)
		return
	}

	// Start span
	attrs := map[string]string{
		"function.name": spanName,
	}
	
	spanCtx, span := tm.StartSpanWithAttributes(ctx, spanName, attrs)
	if span == nil {
		// Span creation failed, execute function normally (zero-error design)
		fn(arg)
		return
	}

	// Measure execution time
	start := time.Now()

	// Execute function
	fn(arg)

	// Record duration
	duration := time.Since(start)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("function.duration_ms", duration.Milliseconds()),
			attribute.Int64("function.duration_ns", duration.Nanoseconds()),
		)
		span.SetStatus(codes.Ok, "")
	}

	// End span
	span.End()

	// Update context in parser
	parser.SetContext(spanCtx)
}

// TraceFuncWithContext traces a function that takes context and returns (T, error)
// Usage: result, err, newCtx := TraceFuncWithContext(ctx, func(ctx context.Context) (ResultType, error) { ... })
// Returns: (result, error, newContext) - newContext contains the span context for propagation
func TraceFuncWithContext[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error, context.Context) {
	// Auto-detect span name from caller
	_, spanName := getCallerInfo(1)
	if spanName == "" || spanName == "unknown" {
		spanName = "function"
	}

	// Fast path: if tracing is not enabled, execute function without tracing
	if !isTracingEnabled(ctx) {
		result, err := fn(ctx)
		return result, err, ctx
	}

	// Get tracing manager
	tm := GetGlobalTracingManager()
	if tm == nil {
		// Tracing manager not available, execute function normally
		result, err := fn(ctx)
		return result, err, ctx
	}

	// Start span
	attrs := map[string]string{
		"function.name": spanName,
	}

	spanCtx, span := tm.StartSpanWithAttributes(ctx, spanName, attrs)
	if span == nil {
		// Span creation failed, execute function normally (zero-error design)
		result, err := fn(ctx)
		return result, err, ctx
	}

	// Measure execution time
	start := time.Now()

	// Execute function with span context
	result, err := fn(spanCtx)

	// Record duration
	duration := time.Since(start)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("function.duration_ms", duration.Milliseconds()),
			attribute.Int64("function.duration_ns", duration.Nanoseconds()),
		)
	}

	// Record error if any (zero-error: handle internally)
	if err != nil {
		if span.IsRecording() {
			tm.RecordError(spanCtx, err, map[string]string{
				"error.type": "function_error",
			})
			span.SetStatus(codes.Error, err.Error())
		}
	} else {
		if span.IsRecording() {
			span.SetStatus(codes.Ok, "")
		}
	}

	// End span
	span.End()

	return result, err, spanCtx
}

// TraceErrorWithContext traces a function that takes context and returns error
// Usage: err, newCtx := TraceErrorWithContext(ctx, func(ctx context.Context) error { ... })
// Returns: (error, newContext) - newContext contains the span context for propagation
func TraceErrorWithContext(ctx context.Context, fn func(context.Context) error) (error, context.Context) {
	// Auto-detect span name from caller
	_, spanName := getCallerInfo(1)
	if spanName == "" || spanName == "unknown" {
		spanName = "function"
	}

	// Fast path: if tracing is not enabled, execute function without tracing
	if !isTracingEnabled(ctx) {
		err := fn(ctx)
		return err, ctx
	}

	// Get tracing manager
	tm := GetGlobalTracingManager()
	if tm == nil {
		// Tracing manager not available, execute function normally
		err := fn(ctx)
		return err, ctx
	}

	// Start span
	attrs := map[string]string{
		"function.name": spanName,
	}

	spanCtx, span := tm.StartSpanWithAttributes(ctx, spanName, attrs)
	if span == nil {
		// Span creation failed, execute function normally (zero-error design)
		err := fn(ctx)
		return err, ctx
	}

	// Measure execution time
	start := time.Now()

	// Execute function with span context
	err := fn(spanCtx)

	// Record duration
	duration := time.Since(start)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("function.duration_ms", duration.Milliseconds()),
			attribute.Int64("function.duration_ns", duration.Nanoseconds()),
		)
	}

	// Record error if any (zero-error: handle internally)
	if err != nil {
		if span.IsRecording() {
			tm.RecordError(spanCtx, err, map[string]string{
				"error.type": "function_error",
			})
			span.SetStatus(codes.Error, err.Error())
		}
	} else {
		if span.IsRecording() {
			span.SetStatus(codes.Ok, "")
		}
	}

	// End span
	span.End()

	return err, spanCtx
}

// TraceVoidWithContext traces a function that takes context and returns nothing
// Usage: newCtx := TraceVoidWithContext(ctx, func(ctx context.Context) { ... })
// Returns: newContext - contains the span context for propagation
func TraceVoidWithContext(ctx context.Context, fn func(context.Context)) context.Context {
	// Auto-detect span name from caller
	_, spanName := getCallerInfo(1)
	if spanName == "" || spanName == "unknown" {
		spanName = "function"
	}

	// Fast path: if tracing is not enabled, execute function without tracing
	if !isTracingEnabled(ctx) {
		fn(ctx)
		return ctx
	}

	// Get tracing manager
	tm := GetGlobalTracingManager()
	if tm == nil {
		// Tracing manager not available, execute function normally
		fn(ctx)
		return ctx
	}

	// Start span
	attrs := map[string]string{
		"function.name": spanName,
	}

	spanCtx, span := tm.StartSpanWithAttributes(ctx, spanName, attrs)
	if span == nil {
		// Span creation failed, execute function normally (zero-error design)
		fn(ctx)
		return ctx
	}

	// Measure execution time
	start := time.Now()

	// Execute function with span context
	fn(spanCtx)

	// Record duration
	duration := time.Since(start)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("function.duration_ms", duration.Milliseconds()),
			attribute.Int64("function.duration_ns", duration.Nanoseconds()),
		)
		span.SetStatus(codes.Ok, "")
	}

	// End span
	span.End()

	return spanCtx
}

// TraceFuncWithSpanName traces a function that takes context and returns (T, error) with custom span name and attributes
// Usage: result, err, newCtx := TraceFuncWithSpanName(ctx, spanName, attrs, func(ctx context.Context) (ResultType, error) { ... })
func TraceFuncWithSpanName[T any](ctx context.Context, spanName string, attrs map[string]string, fn func(context.Context) (T, error)) (T, error, context.Context) {
	// Fast path: if tracing is not enabled, execute function without tracing
	if !isTracingEnabled(ctx) {
		result, err := fn(ctx)
		return result, err, ctx
	}

	// Get tracing manager
	tm := GetGlobalTracingManager()
	if tm == nil {
		// Tracing manager not available, execute function normally
		result, err := fn(ctx)
		return result, err, ctx
	}

	// Use provided span name or auto-detect
	if spanName == "" {
		_, spanName = getCallerInfo(1)
		if spanName == "" || spanName == "unknown" {
			spanName = "function"
		}
	}

	// Merge provided attributes with default
	spanAttrs := make(map[string]string)
	for k, v := range attrs {
		spanAttrs[k] = v
	}
	spanAttrs["function.name"] = spanName

	spanCtx, span := tm.StartSpanWithAttributes(ctx, spanName, spanAttrs)
	if span == nil {
		// Span creation failed, execute function normally (zero-error design)
		result, err := fn(ctx)
		return result, err, ctx
	}

	// Measure execution time
	start := time.Now()

	// Execute function with span context
	result, err := fn(spanCtx)

	// Record duration
	duration := time.Since(start)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("function.duration_ms", duration.Milliseconds()),
			attribute.Int64("function.duration_ns", duration.Nanoseconds()),
		)
	}

	// Record error if any (zero-error: handle internally)
	if err != nil {
		if span.IsRecording() {
			tm.RecordError(spanCtx, err, map[string]string{
				"error.type": "function_error",
			})
			span.SetStatus(codes.Error, err.Error())
		}
	} else {
		if span.IsRecording() {
			span.SetStatus(codes.Ok, "")
		}
	}

	// End span
	span.End()

	return result, err, spanCtx
}
