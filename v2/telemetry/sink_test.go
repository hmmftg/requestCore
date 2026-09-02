package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// captureHandler is a test slog.Handler that captures all log records.
type captureHandler struct {
	records []captureRecord
}

type captureRecord struct {
	level   slog.Level
	message string
	attrs   []slog.Attr
}

func (h *captureHandler) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make([]slog.Attr, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		// Resolve LogValuer values to mimic real slog handlers.
		rv := a.Value.Resolve()
		if rv.Any() != nil {
			a = slog.Any(a.Key, rv.Any())
		}
		attrs = append(attrs, a)
		return true
	})
	h.records = append(h.records, captureRecord{
		level:   r.Level,
		message: r.Message,
		attrs:   attrs,
	})
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(name string) slog.Handler       { return h }

func TestEventType_String(t *testing.T) {
	tests := []struct {
		e    EventType
		want string
	}{
		{EventStart, "start"},
		{EventSuccess, "success"},
		{EventFailure, "failure"},
		{EventBusiness, "business"},
	}
	for _, tt := range tests {
		if got := tt.e.String(); got != tt.want {
			t.Errorf("EventType(%d).String() = %q, want %q", tt.e, got, tt.want)
		}
	}
}

func TestEventType_Level(t *testing.T) {
	if EventFailure.Level() != slog.LevelError {
		t.Fatal("expected Error level for failure")
	}
	if EventSuccess.Level() != slog.LevelInfo {
		t.Fatal("expected Info level for success")
	}
	if EventStart.Level() != slog.LevelInfo {
		t.Fatal("expected Info level for start")
	}
}

func TestNopSink(t *testing.T) {
	s := NopSink{}
	s.Record(Event{Type: EventSuccess, Status: 200})
	// No panic, no output expected.
}

func TestSlogSink_NilLoggerDefaults(t *testing.T) {
	s := NewSlogSink(nil)
	if s == nil {
		t.Fatal("expected non-nil sink")
	}
	// Should not panic when recording.
	s.Record(Event{Type: EventSuccess, Status: 200})
}

func TestSlogSink_RecordSuccess(t *testing.T) {
	h := &captureHandler{}
	s := NewSlogSink(slog.New(h))

	s.Record(Event{
		Type:         EventSuccess,
		Operation:    "createUser",
		Method:       "POST",
		RoutePattern: "/users",
		Status:       201,
		Duration:     5 * time.Millisecond,
		RequestID:    "req-123",
		TraceID:      "trace-456",
	})

	if len(h.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(h.records))
	}
	rec := h.records[0]
	if rec.level != slog.LevelInfo {
		t.Fatalf("expected Info level, got %v", rec.level)
	}
	if rec.message != "success" {
		t.Fatalf("expected message 'success', got %q", rec.message)
	}

	// Verify attributes.
	attrMap := attrsToMap(rec.attrs)
	if attrMap["operation"] != "createUser" {
		t.Fatalf("expected operation createUser, got %v", attrMap["operation"])
	}
	if attrMap["method"] != "POST" {
		t.Fatalf("expected method POST, got %v", attrMap["method"])
	}
	if attrMap["route"] != "/users" {
		t.Fatalf("expected route /users, got %v", attrMap["route"])
	}
	if attrMap["status"] != int64(201) {
		t.Fatalf("expected status 201, got %v", attrMap["status"])
	}
	if attrMap["request_id"] != "req-123" {
		t.Fatalf("expected request_id, got %v", attrMap["request_id"])
	}
	if attrMap["trace_id"] != "trace-456" {
		t.Fatalf("expected trace_id, got %v", attrMap["trace_id"])
	}
}

func TestSlogSink_RecordFailure(t *testing.T) {
	h := &captureHandler{}
	s := NewSlogSink(slog.New(h))

	testErr := errors.New("db connection lost")
	s.Record(Event{
		Type:      EventFailure,
		Operation: "getUser",
		Method:    "GET",
		Status:    500,
		Err:       testErr,
	})

	if len(h.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(h.records))
	}
	rec := h.records[0]
	if rec.level != slog.LevelError {
		t.Fatalf("expected Error level, got %v", rec.level)
	}
	attrMap := attrsToMap(rec.attrs)
	if attrMap["error"] != "db connection lost" {
		t.Fatalf("expected error attr, got %v", attrMap["error"])
	}
}

func TestSlogSink_BodyExclusion(t *testing.T) {
	h := &captureHandler{}
	s := NewSlogSink(slog.New(h))

	// Even if attrs contain body-like data, the sink itself does not
	// add request or response bodies. It only adds the safe metadata.
	s.Record(Event{
		Type:      EventSuccess,
		Operation: "createUser",
		Status:    201,
		Attrs:     []slog.Attr{slog.String("cache", "hit")},
	})

	rec := h.records[0]
	rendered := recordToString(rec)
	// Verify no "body" or "request_body" or "response_body" keys.
	if strings.Contains(rendered, "body") {
		t.Fatalf("expected no body in output: %s", rendered)
	}
	// Verify the custom attr is present.
	attrMap := attrsToMap(rec.attrs)
	if attrMap["cache"] != "hit" {
		t.Fatalf("expected cache=hit, got %v", attrMap["cache"])
	}
}

func TestSlogSink_EmptyEvent(t *testing.T) {
	h := &captureHandler{}
	s := NewSlogSink(slog.New(h))

	s.Record(Event{Type: EventStart})

	if len(h.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(h.records))
	}
	rec := h.records[0]
	if rec.message != "start" {
		t.Fatalf("expected message 'start', got %q", rec.message)
	}
	// With empty fields, only the message should be logged (no attrs).
	if len(rec.attrs) != 0 {
		t.Fatalf("expected 0 attrs for empty event, got %d", len(rec.attrs))
	}
}

func TestMultiSink_FanOut(t *testing.T) {
	var count1, count2 int
	s1 := &countingSink{count: &count1}
	s2 := &countingSink{count: &count2}

	m := NewMultiSink(s1, s2)
	m.Record(Event{Type: EventSuccess, Status: 200})
	m.Record(Event{Type: EventFailure, Status: 500})

	if count1 != 2 {
		t.Fatalf("expected sink1 count 2, got %d", count1)
	}
	if count2 != 2 {
		t.Fatalf("expected sink2 count 2, got %d", count2)
	}
}

func TestMultiSink_NilSinksFiltered(t *testing.T) {
	m := NewMultiSink(nil, NopSink{}, nil)
	if m.Len() != 1 {
		t.Fatalf("expected 1 sink, got %d", m.Len())
	}
}

func TestMultiSink_Add(t *testing.T) {
	m := NewMultiSink()
	if m.Len() != 0 {
		t.Fatalf("expected 0 sinks, got %d", m.Len())
	}
	m.Add(NopSink{})
	if m.Len() != 1 {
		t.Fatalf("expected 1 sink, got %d", m.Len())
	}
	m.Add(nil)
	if m.Len() != 1 {
		t.Fatalf("expected still 1 sink, got %d", m.Len())
	}
}

func TestMultiSink_PanicIsolation(t *testing.T) {
	var count int
	normalSink := &countingSink{count: &count}
	panicSink := &panickingSink{}

	m := NewMultiSink(panicSink, normalSink)
	// Should not panic despite panicSink.
	m.Record(Event{Type: EventSuccess, Status: 200})

	if count != 1 {
		t.Fatalf("expected normal sink to still receive event, got count %d", count)
	}
}

// countingSink is a test Sink that counts Record calls.
type countingSink struct {
	count *int
}

func (s *countingSink) Record(Event) {
	*s.count++
}

// panickingSink is a test Sink that panics on Record.
type panickingSink struct{}

func (s *panickingSink) Record(Event) {
	panic("sink panic")
}

// attrsToMap converts a slice of slog.Attr to a map for easy testing.
func attrsToMap(attrs []slog.Attr) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value.Any()
	}
	return m
}

// recordToString renders a captureRecord for string matching tests.
func recordToString(r captureRecord) string {
	var sb strings.Builder
	sb.WriteString(r.message)
	for _, a := range r.attrs {
		sb.WriteString(" ")
		sb.WriteString(a.Key)
		sb.WriteString("=")
		sb.WriteString(a.Value.String())
	}
	return sb.String()
}
