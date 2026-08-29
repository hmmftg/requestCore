package telemetry

import "sync"

// MultiSink is a Sink that fans out events to multiple underlying sinks.
// It is safe for concurrent use.
type MultiSink struct {
	mu     sync.RWMutex
	sinks  []Sink
}

// NewMultiSink creates a MultiSink wrapping the given sinks. Nil sinks
// are filtered out.
func NewMultiSink(sinks ...Sink) *MultiSink {
	filtered := make([]Sink, 0, len(sinks))
	for _, s := range sinks {
		if s != nil {
			filtered = append(filtered, s)
		}
	}
	return &MultiSink{sinks: filtered}
}

// Record fans out the event to all underlying sinks. A panic in one
// sink does not prevent other sinks from receiving the event.
func (m *MultiSink) Record(event Event) {
	m.mu.RLock()
	sinks := m.sinks
	m.mu.RUnlock()
	for _, s := range sinks {
		// Each sink is called in order; a panic in one should not
		// prevent others from receiving the event.
		func() {
			defer func() { _ = recover() }()
			s.Record(event)
		}()
	}
}

// Add appends a sink to the multi-sink. Nil sinks are ignored.
func (m *MultiSink) Add(sink Sink) {
	if sink == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sinks = append(m.sinks, sink)
}

// Len returns the number of underlying sinks.
func (m *MultiSink) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sinks)
}
