package response

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/v2/request"
)

// Regression tests for Phase 2: 201 + Location, 204 no body, HEAD
// body suppression, and mapper sanitization.

func TestWriteSuccess_201WithLocation(t *testing.T) {
	ctx := request.NewContext(context.TODO())
	transport := &helpersTransport{}

	ctx.Response().AddHeader("Location", "/resources/42")
	body := []byte(`{"id":42,"name":"created"}`)
	if err := WriteSuccess(ctx, transport, http.StatusCreated, "application/json", body); err != nil {
		t.Fatalf("WriteSuccess failed: %v", err)
	}
	if transport.status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", transport.status)
	}
	if transport.headers.Get("Location") != "/resources/42" {
		t.Fatalf("expected Location /resources/42, got %q", transport.headers.Get("Location"))
	}
	if string(transport.body) != `{"id":42,"name":"created"}` {
		t.Fatalf("expected body, got %q", string(transport.body))
	}
}

func TestWriteSuccess_204NoBody(t *testing.T) {
	ctx := request.NewContext(context.TODO())
	transport := &helpersTransport{}

	// 204 should suppress the body even if one is provided
	if err := WriteSuccess(ctx, transport, http.StatusNoContent, "application/json", []byte(`{}`)); err != nil {
		t.Fatalf("WriteSuccess failed: %v", err)
	}
	if transport.status != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", transport.status)
	}
	if transport.body != nil {
		t.Fatalf("expected nil body for 204, got %q", string(transport.body))
	}
}

func TestNoContent_NoBody(t *testing.T) {
	ctx := request.NewContext(context.TODO())
	transport := &helpersTransport{}

	if err := NoContent(ctx, transport); err != nil {
		t.Fatalf("NoContent failed: %v", err)
	}
	if transport.status != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", transport.status)
	}
	if transport.body != nil {
		t.Fatalf("expected nil body, got %v", transport.body)
	}
}

func TestWriteSuccess_HEADSuppressesBody(t *testing.T) {
	ctx := request.NewContext(context.TODO())
	transport := &helpersTransport{}

	// Simulate HEAD request by suppressing the body via response state
	ctx.Response().SuppressBody()
	body := []byte(`{"data":"should be suppressed"}`)
	if err := WriteSuccess(ctx, transport, http.StatusOK, "application/json", body); err != nil {
		t.Fatalf("WriteSuccess failed: %v", err)
	}
	if transport.status != http.StatusOK {
		t.Fatalf("expected 200, got %d", transport.status)
	}
	// HEAD requests should suppress the body
	if transport.body != nil {
		t.Fatalf("expected nil body for HEAD, got %q", string(transport.body))
	}
}

func TestMapperSanitization_UnknownError(t *testing.T) {
	r := NewMapperRegistry()
	p := r.Map(errors.New("database password is secret123 and the connection string is postgres://user:pass@host:5432/db"))

	if p.Status != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", p.Status)
	}
	if p.Title != "Internal Server Error" {
		t.Fatalf("expected 'Internal Server Error', got %q", p.Title)
	}
	// The detail must not contain the raw error message
	if p.Detail != "" {
		t.Fatalf("expected empty detail (sanitized), got %q", p.Detail)
	}
	// Verify the raw error is not in the JSON output
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	str := string(body)
	if strings.Contains(str, "secret123") {
		t.Fatalf("raw error leaked into JSON: %s", str)
	}
	if strings.Contains(str, "postgres://user:pass") {
		t.Fatalf("connection string leaked into JSON: %s", str)
	}
}

func TestMapperSanitization_CauseNeverSerialized(t *testing.T) {
	r := NewMapperRegistry()
	_ = r.Register(
		func(_ error) bool { return true },
		func(err error) *Problem {
			return NewProblem(http.StatusConflict, "Conflict").
				WithDetail("duplicate resource").
				WithCause(err)
		},
	)

	cause := errors.New("internal: password=hunter2, token=abc123")
	p := r.Map(cause)

	body, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	str := string(body)
	if strings.Contains(str, "hunter2") {
		t.Fatalf("cause leaked into JSON: %s", str)
	}
	if strings.Contains(str, "abc123") {
		t.Fatalf("cause leaked into JSON: %s", str)
	}
	if strings.Contains(str, "password") {
		t.Fatalf("cause leaked into JSON: %s", str)
	}
}
