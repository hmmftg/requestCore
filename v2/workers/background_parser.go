package workers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore/webFramework"
)

// BackgroundParser is a concurrency-safe implementation of the root
// webFramework.RequestParser interface for worker jobs. It has no HTTP
// request context; most methods are no-ops inherited from FakeParser.
//
// The critical methods — GetLocal, SetLocal, AddCustomAttributes — are
// overridden with mutex-protected versions that safely store AddLog
// entries and forward custom attributes to the TransactionSink for the
// Splunk transaction pipeline.
//
// This parser satisfies the full root webFramework.RequestParser contract
// so that webFramework.AddLog, webFramework.CollectLogArrays, and
// handlers.CallAPI work directly with no adapters or type assertions.
type BackgroundParser struct {
	webFramework.FakeParser

	mu     sync.Mutex
	locals map[string]any
	sink   *TransactionSink

	// ctx is the job context for tracing propagation.
	ctx context.Context
}

// newBackgroundParser creates a BackgroundParser bound to the given sink
// and context.
func newBackgroundParser(sink *TransactionSink, ctx context.Context) *BackgroundParser {
	return &BackgroundParser{
		locals: make(map[string]any),
		sink:   sink,
		ctx:    ctx,
	}
}

// GetLocal returns a value from local storage by name (concurrency-safe).
func (p *BackgroundParser) GetLocal(name string) any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.locals[name]
}

// GetLocalString returns a local value as a string (concurrency-safe).
func (p *BackgroundParser) GetLocalString(name string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, ok := p.locals[name]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// SetLocal stores a value in local storage by name (concurrency-safe).
// This is the method called by webFramework.AddLog to store log entries.
func (p *BackgroundParser) SetLocal(name string, value any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.locals[name] = value
}

// AddCustomAttributes adds a log attribute to the transaction sink.
// This is called by webFramework.CollectLogArrays/CollectLogTags to
// flush collected log entries into the Splunk transaction pipeline.
func (p *BackgroundParser) AddCustomAttributes(attr slog.Attr) {
	if p.sink != nil {
		p.sink.Add(attr)
	}
}

// GetContext returns the job context for tracing propagation.
func (p *BackgroundParser) GetContext() context.Context {
	if p.ctx != nil {
		return p.ctx
	}
	return context.Background()
}

// SetContext updates the job context.
func (p *BackgroundParser) SetContext(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ctx = ctx
}

// GetTraceContext returns the trace span context from the parser.
func (p *BackgroundParser) GetTraceContext() trace.SpanContext {
	return trace.SpanContext{}
}

// SetTraceContext is a no-op for the background parser.
func (p *BackgroundParser) SetTraceContext(spanCtx trace.SpanContext) {}

// StartSpan returns a no-op span for the background parser.
func (p *BackgroundParser) StartSpan(name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	ctx := p.GetContext()
	return ctx, trace.SpanFromContext(ctx)
}

// Ensure BackgroundParser satisfies the root webFramework.RequestParser.
var _ webFramework.RequestParser = (*BackgroundParser)(nil)
