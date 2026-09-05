package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/v2/endpoint"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/telemetry"
)

// capturingSink is a telemetry.Sink that captures events for test
// assertions. It implements telemetry.Sink.
type capturingSink struct {
	events []telemetry.Event
}

func (s *capturingSink) Record(e telemetry.Event) {
	s.events = append(s.events, e)
}

// TestTelemetry_SuccessPathEmitsStartAndSuccess verifies that the
// executor emits start and success telemetry events on the happy path.
// The canonical event names are <operation>-req for success.
func TestTelemetry_SuccessPathEmitsStartAndSuccess(t *testing.T) {
	sink := &capturingSink{}
	exec := endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithTelemetrySink(sink),
	)
	router := testRouter()

	ep := Get[struct{}, TestResp]("telemetry-ok", "/telemetry-ok",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/telemetry-ok")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify telemetry events were emitted.
	if len(sink.events) < 2 {
		t.Fatalf("expected at least 2 telemetry events, got %d", len(sink.events))
	}

	// First event should be a start event.
	if sink.events[0].Type != telemetry.EventStart {
		t.Fatalf("first event type = %v, want EventStart", sink.events[0].Type)
	}
	if sink.events[0].Operation != "telemetry-ok-req" {
		t.Fatalf("first event operation = %q, want %q", sink.events[0].Operation, "telemetry-ok-req")
	}

	// Last event should be a success event.
	last := sink.events[len(sink.events)-1]
	if last.Type != telemetry.EventSuccess {
		t.Fatalf("last event type = %v, want EventSuccess", last.Type)
	}
	if last.Operation != "telemetry-ok-req" {
		t.Fatalf("last event operation = %q, want %q", last.Operation, "telemetry-ok-req")
	}
	if last.Status != http.StatusOK {
		t.Fatalf("last event status = %d, want %d", last.Status, http.StatusOK)
	}
	if last.Err != nil {
		t.Fatalf("last event err = %v, want nil", last.Err)
	}
}

// TestTelemetry_FailurePathEmitsFailure verifies that a handler error
// emits a failure telemetry event with the error.
func TestTelemetry_FailurePathEmitsFailure(t *testing.T) {
	sink := &capturingSink{}
	exec := endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithTelemetrySink(sink),
	)
	router := testRouter()

	ep := Get[struct{}, TestResp]("telemetry-fail", "/telemetry-fail",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{}, errors.New("handler failure")
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/telemetry-fail")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected non-200 status, got 200")
	}

	// Find the failure event.
	var failureEvent *telemetry.Event
	for i := range sink.events {
		if sink.events[i].Type == telemetry.EventFailure {
			failureEvent = &sink.events[i]
			break
		}
	}
	if failureEvent == nil {
		t.Fatal("expected a failure telemetry event, got none")
	}
	if failureEvent.Operation != "telemetry-fail-req-failed" {
		t.Fatalf("failure event operation = %q, want %q", failureEvent.Operation, "telemetry-fail-req-failed")
	}
	if failureEvent.Err == nil {
		t.Fatal("expected non-nil error in failure event")
	}
}

// TestTelemetry_PanicPathEmitsFailure verifies that a handler panic
// emits a failure telemetry event.
func TestTelemetry_PanicPathEmitsFailure(t *testing.T) {
	sink := &capturingSink{}
	exec := endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithTelemetrySink(sink),
	)
	router := testRouter()

	ep := Get[struct{}, TestResp]("telemetry-panic", "/telemetry-panic",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			panic("boom")
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/telemetry-panic")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	// Find the failure event.
	var failureEvent *telemetry.Event
	for i := range sink.events {
		if sink.events[i].Type == telemetry.EventFailure {
			failureEvent = &sink.events[i]
			break
		}
	}
	if failureEvent == nil {
		t.Fatal("expected a failure telemetry event for panic, got none")
	}
}

// TestTelemetry_BindErrorEmitsFailure verifies that a binding error
// emits a failure telemetry event.
func TestTelemetry_BindErrorEmitsFailure(t *testing.T) {
	sink := &capturingSink{}
	exec := endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithTelemetrySink(sink),
	)
	router := testRouter()

	ep := Post[CreateReq, CreateResp]("telemetry-bind-err", "/telemetry-bind-err",
		func(ctx *request.Context, req CreateReq) (CreateResp, error) {
			return CreateResp{ID: "1", Name: req.Name}, nil
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	// Send invalid JSON to trigger a binding error.
	resp, err := http.Post(srv.URL+"/telemetry-bind-err", "application/json", strings.NewReader(`{invalid`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected non-200 status, got 200")
	}

	// Find the failure event.
	var failureEvent *telemetry.Event
	for i := range sink.events {
		if sink.events[i].Type == telemetry.EventFailure {
			failureEvent = &sink.events[i]
			break
		}
	}
	if failureEvent == nil {
		t.Fatal("expected a failure telemetry event for bind error, got none")
	}
}

// TestTelemetry_NopSinkDoesNotPanic verifies that a nil sink (default
// executor) does not panic during request processing.
func TestTelemetry_NopSinkDoesNotPanic(t *testing.T) {
	exec := testExecutor() // uses WithNopTelemetry
	router := testRouter()

	ep := Get[struct{}, TestResp]("nop-sink", "/nop-sink",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nop-sink")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// TestTelemetry_SuccessResponseBodyNotInEvent verifies that the
// telemetry event does not include raw response bodies. The Event
// struct has no body field; only the Err field carries data on failure.
func TestTelemetry_SuccessResponseBodyNotInEvent(t *testing.T) {
	sink := &capturingSink{}
	exec := endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithTelemetrySink(sink),
	)
	router := testRouter()

	ep := Get[struct{}, TestResp]("no-body-leak", "/no-body-leak",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{Status: "secret-value"}, nil
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/no-body-leak")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	// The HTTP response body should contain the secret (it's the
	// actual response, not telemetry).
	var result TestResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != "secret-value" {
		t.Fatalf("expected 'secret-value' in response, got %q", result.Status)
	}

	// Verify no telemetry event contains the secret in any field.
	for _, e := range sink.events {
		// The Event struct has no body field, but check Err for
		// safety — on success Err is nil.
		if e.Err != nil && strings.Contains(e.Err.Error(), "secret-value") {
			t.Fatalf("telemetry event error contains response body: %v", e.Err)
		}
	}
}
