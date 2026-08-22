package workers

import (
	"log/slog"
	"sync"
)

// backgroundParser is a concurrency-safe Parser implementation for worker
// jobs. It stores AddLog entries in a TransactionSink and supports
// AddCustomAttributes for the Splunk transaction pipeline.
type backgroundParser struct {
	mu    sync.Mutex
	locals map[string]any
	sink   *TransactionSink
}

// newBackgroundParser creates a backgroundParser bound to the given sink.
func newBackgroundParser(sink *TransactionSink) *backgroundParser {
	return &backgroundParser{
		locals: make(map[string]any),
		sink:   sink,
	}
}

// GetLocal returns a value from local storage by name.
func (p *backgroundParser) GetLocal(name string) any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.locals[name]
}

// SetLocal stores a value in local storage by name.
func (p *backgroundParser) SetLocal(name string, value any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.locals[name] = value
}

// AddCustomAttributes adds a log attribute to the transaction sink.
// This mirrors the v1 webFramework.RequestParser.AddCustomAttributes
// behavior so AddLog entries flow into the Splunk pipeline.
func (p *backgroundParser) AddCustomAttributes(attr slog.Attr) {
	if p.sink != nil {
		p.sink.Add(attr)
	}
}

// jobWebFramework implements the WebFramework interface for worker jobs.
type jobWebFramework struct {
	parser *backgroundParser
}

// Parser returns the background parser for AddLog calls.
func (w *jobWebFramework) Parser() Parser {
	return w.parser
}
