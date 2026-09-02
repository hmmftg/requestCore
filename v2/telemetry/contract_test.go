package telemetry

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// contract_test.go contains the Phase 0 observability contract tests
// required by the Tranche 4 plan. They prove:
//   - success events include the safely projected response (never raw bodies)
//   - failure events include the error
//   - slog.LogValuer masking is honored
//   - a panicking LogValuer never falls back to raw, unmasked data
//
// These tests double as the regression gate for the global rule's v2
// observability carve-out: if any of them breaks, the v2 slog path must
// not be considered a safe replacement for the v1 AddLog bridge.

// safeResponse is a projected response value that exposes only safe
// metadata. It implements slog.LogValuer so sensitive fields are masked
// before reaching the sink.
type safeResponse struct {
	ID     string
	Status string
	// Secret is never logged.
	Secret string
}

// LogValue implements slog.LogValuer. It returns a group containing only
// the safe fields, masking Secret.
func (s safeResponse) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", s.ID),
		slog.String("status", s.Status),
		slog.String("secret", "***masked***"),
	)
}

func TestContract_SuccessIncludesProjectedResponse(t *testing.T) {
	h := &captureHandler{}
	s := NewSlogSink(slog.New(h))

	resp := safeResponse{ID: "u-1", Status: "active", Secret: "hunter2"}
	s.Record(Event{
		Type:      EventSuccess,
		Operation: "getUser",
		Status:    200,
		Attrs:     []slog.Attr{slog.Any("response", resp)},
	})

	if len(h.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(h.records))
	}
	rec := h.records[0]
	rendered := recordToString(rec)

	// Safe projected fields must be present.
	if !strings.Contains(rendered, "u-1") {
		t.Fatalf("expected projected id in output: %s", rendered)
	}
	if !strings.Contains(rendered, "active") {
		t.Fatalf("expected projected status in output: %s", rendered)
	}
	// Raw secret must never appear; only the masked placeholder.
	if strings.Contains(rendered, "hunter2") {
		t.Fatalf("raw secret leaked into telemetry output: %s", rendered)
	}
	if !strings.Contains(rendered, "***masked***") {
		t.Fatalf("expected masked secret placeholder: %s", rendered)
	}
}

func TestContract_FailureIncludesError(t *testing.T) {
	h := &captureHandler{}
	s := NewSlogSink(slog.New(h))

	busErr := errors.New("payment gateway timeout")
	s.Record(Event{
		Type:      EventFailure,
		Operation: "chargeCard",
		Status:    502,
		Err:       busErr,
	})

	if len(h.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(h.records))
	}
	rec := h.records[0]
	if rec.level != slog.LevelError {
		t.Fatalf("expected Error level, got %v", rec.level)
	}
	attrMap := attrsToMap(rec.attrs)
	if attrMap["error"] != "payment gateway timeout" {
		t.Fatalf("expected error attr 'payment gateway timeout', got %v", attrMap["error"])
	}
}

// maskingLogValuer masks a sensitive field via LogValuer.
type maskingLogValuer struct {
	Token string
}

func (m maskingLogValuer) LogValue() slog.Value {
	return slog.GroupValue(slog.String("token", "***masked***"))
}

func TestContract_LogValuerMaskingHonored(t *testing.T) {
	h := &captureHandler{}
	s := NewSlogSink(slog.New(h))

	v := maskingLogValuer{Token: "super-secret-token"}
	s.Record(Event{
		Type:      EventSuccess,
		Operation: "login",
		Attrs:     []slog.Attr{slog.Any("creds", v)},
	})

	rec := h.records[0]
	rendered := recordToString(rec)
	if strings.Contains(rendered, "super-secret-token") {
		t.Fatalf("raw token leaked through LogValuer masking: %s", rendered)
	}
	if !strings.Contains(rendered, "***masked***") {
		t.Fatalf("expected masked token placeholder: %s", rendered)
	}
}

// panickingLogValuer panics during LogValue resolution. Its underlying
// Go value contains a raw secret that must never reach the sink.
type panickingLogValuer struct {
	RawSecret string
}

func (p panickingLogValuer) LogValue() slog.Value {
	panic("LogValuer boom")
}

func TestContract_PanickingLogValuerNoRawFallback(t *testing.T) {
	h := &captureHandler{}
	s := NewSlogSink(slog.New(h))

	v := panickingLogValuer{RawSecret: "must-not-leak"}
	// Recording must not panic.
	s.Record(Event{
		Type:      EventSuccess,
		Operation: "risky",
		Attrs:     []slog.Attr{slog.Any("payload", v)},
	})

	if len(h.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(h.records))
	}
	rec := h.records[0]
	rendered := recordToString(rec)
	// The raw secret must never appear even when the LogValuer panics.
	if strings.Contains(rendered, "must-not-leak") {
		t.Fatalf("raw secret leaked after panicking LogValuer: %s", rendered)
	}
}

// TestContract_BodiesNeverIncluded asserts that even when a caller
// accidentally passes body-like attribute keys, the sink itself does not
// add request or response bodies. (The sink only emits the safe metadata
// fields plus caller-supplied Attrs; this test documents that contract.)
func TestContract_BodiesNeverIncludedBySink(t *testing.T) {
	h := &captureHandler{}
	s := NewSlogSink(slog.New(h))

	s.Record(Event{
		Type:      EventSuccess,
		Operation: "createUser",
		Method:    "POST",
		Status:    201,
	})
	rec := h.records[0]
	rendered := recordToString(rec)
	for _, banned := range []string{"body", "request_body", "response_body", "req_body", "resp_body"} {
		if strings.Contains(rendered, banned) {
			t.Fatalf("sink emitted banned body key %q: %s", banned, rendered)
		}
	}
}

// Ensure the captureHandler used by these tests is the same one defined
// in sink_test.go (package telemetry). The compiler will fail if it was
// renamed or removed, keeping the contract tests honest.
var _ slog.Handler = (*captureHandler)(nil)
