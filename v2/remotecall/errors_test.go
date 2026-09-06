package remotecall_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/hmmftg/requestCore/v2/remotecall"
	"github.com/hmmftg/requestCore/v2/response"
)

func TestErrorKind_String(t *testing.T) {
	tests := []struct {
		kind     remotecall.ErrorKind
		expected string
	}{
		{remotecall.ErrorKindHTTP, "http"},
		{remotecall.ErrorKindTransport, "transport"},
		{remotecall.ErrorKindTimeout, "timeout"},
		{remotecall.ErrorKindContext, "context"},
		{remotecall.ErrorKindDecode, "decode"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.expected {
			t.Errorf("ErrorKind(%d).String() = %q, want %q", tt.kind, got, tt.expected)
		}
	}
}

func TestRemoteCallError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *remotecall.RemoteCallError
		contains string
	}{
		{
			name: "http error",
			err: &remotecall.RemoteCallError{
				Kind:         remotecall.ErrorKindHTTP,
				StatusCode:   500,
				ResponseBody: []byte(`{"error":"foo"}`),
			},
			contains: "http 500",
		},
		{
			name: "transport error",
			err: &remotecall.RemoteCallError{
				Kind: remotecall.ErrorKindTransport,
				Err:  errors.New("connection refused"),
			},
			contains: "transport",
		},
		{
			name: "timeout error",
			err: &remotecall.RemoteCallError{
				Kind: remotecall.ErrorKindTimeout,
				Err:  context.DeadlineExceeded,
			},
			contains: "timeout",
		},
		{
			name: "context error",
			err: &remotecall.RemoteCallError{
				Kind: remotecall.ErrorKindContext,
				Err:  context.Canceled,
			},
			contains: "context",
		},
		{
			name: "decode error",
			err: &remotecall.RemoteCallError{
				Kind: remotecall.ErrorKindDecode,
				Err:  errors.New("invalid json"),
			},
			contains: "decode",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			if !contains(msg, tt.contains) {
				t.Errorf("Error() = %q, expected to contain %q", msg, tt.contains)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRemoteCallError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	rce := &remotecall.RemoteCallError{
		Kind: remotecall.ErrorKindTransport,
		Err:  inner,
	}
	if !errors.Is(rce, inner) {
		t.Error("errors.Is should find inner error")
	}
}

func TestIsHTTPError(t *testing.T) {
	rce := &remotecall.RemoteCallError{Kind: remotecall.ErrorKindHTTP}
	if !remotecall.IsHTTPError(rce) {
		t.Error("expected IsHTTPError to be true")
	}
	if remotecall.IsTransportError(rce) {
		t.Error("expected IsTransportError to be false")
	}
}

func TestIsTransportError(t *testing.T) {
	rce := &remotecall.RemoteCallError{Kind: remotecall.ErrorKindTransport}
	if !remotecall.IsTransportError(rce) {
		t.Error("expected IsTransportError to be true")
	}
}

func TestIsTimeoutError(t *testing.T) {
	rce := &remotecall.RemoteCallError{Kind: remotecall.ErrorKindTimeout}
	if !remotecall.IsTimeoutError(rce) {
		t.Error("expected IsTimeoutError to be true")
	}
}

func TestIsContextCancelled(t *testing.T) {
	rce := &remotecall.RemoteCallError{Kind: remotecall.ErrorKindContext}
	if !remotecall.IsContextCancelled(rce) {
		t.Error("expected IsContextCancelled to be true")
	}
}

func TestIsDecodeError(t *testing.T) {
	rce := &remotecall.RemoteCallError{Kind: remotecall.ErrorKindDecode}
	if !remotecall.IsDecodeError(rce) {
		t.Error("expected IsDecodeError to be true")
	}
}

func TestToProblem_ProblemJSON(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/problem+json")
	body := []byte(`{"type":"https://example.com/errors/not-found","title":"Not Found","status":404,"detail":"Resource not found","instance":"/orders/123"}`)

	rce := &remotecall.RemoteCallError{
		Kind:         remotecall.ErrorKindHTTP,
		StatusCode:   404,
		ResponseBody: body,
		Headers:      headers,
	}

	problem, err := rce.ToProblem()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if problem.Status != 404 {
		t.Errorf("expected status 404, got %d", problem.Status)
	}
	if problem.Title != "Not Found" {
		t.Errorf("expected title 'Not Found', got %q", problem.Title)
	}
	if problem.Detail != "Resource not found" {
		t.Errorf("expected detail, got %q", problem.Detail)
	}
	if problem.Instance != "/orders/123" {
		t.Errorf("expected instance, got %q", problem.Instance)
	}
}

func TestToProblem_ProblemJSONWithCharset(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/problem+json; charset=utf-8")
	body := []byte(`{"type":"about:blank","title":"Bad Request","status":400}`)

	rce := &remotecall.RemoteCallError{
		Kind:         remotecall.ErrorKindHTTP,
		StatusCode:   400,
		ResponseBody: body,
		Headers:      headers,
	}

	problem, err := rce.ToProblem()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if problem.Status != 400 {
		t.Errorf("expected status 400, got %d", problem.Status)
	}
	if problem.Title != "Bad Request" {
		t.Errorf("expected title 'Bad Request', got %q", problem.Title)
	}
}

func TestToProblem_NonProblemJSON(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	body := []byte(`{"error":"something went wrong"}`)

	rce := &remotecall.RemoteCallError{
		Kind:         remotecall.ErrorKindHTTP,
		StatusCode:   500,
		ResponseBody: body,
		Headers:      headers,
	}

	problem, err := rce.ToProblem()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if problem.Status != 500 {
		t.Errorf("expected status 500, got %d", problem.Status)
	}
	if problem.Title != http.StatusText(500) {
		t.Errorf("expected title %q, got %q", http.StatusText(500), problem.Title)
	}
	if problem.Detail != string(body) {
		t.Errorf("expected detail to be body, got %q", problem.Detail)
	}
}

func TestToProblem_EmptyBody(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	rce := &remotecall.RemoteCallError{
		Kind:         remotecall.ErrorKindHTTP,
		StatusCode:   503,
		ResponseBody: nil,
		Headers:      headers,
	}

	problem, err := rce.ToProblem()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if problem.Status != 503 {
		t.Errorf("expected status 503, got %d", problem.Status)
	}
	if problem.Detail != "" {
		t.Errorf("expected empty detail, got %q", problem.Detail)
	}
}

func TestToProblem_NonHTTPError(t *testing.T) {
	rce := &remotecall.RemoteCallError{
		Kind: remotecall.ErrorKindTransport,
		Err:  errors.New("connection refused"),
	}

	_, err := rce.ToProblem()
	if err == nil {
		t.Fatal("expected error for non-HTTP ToProblem")
	}
}

func TestToProblem_BinaryBody(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/octet-stream")
	body := []byte{0x00, 0x01, 0x02, 0x00, 0x04} // contains null bytes

	rce := &remotecall.RemoteCallError{
		Kind:         remotecall.ErrorKindHTTP,
		StatusCode:   500,
		ResponseBody: body,
		Headers:      headers,
	}

	problem, err := rce.ToProblem()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if problem.Detail != "" {
		t.Errorf("expected empty detail for binary body, got %q", problem.Detail)
	}
}

func TestToProblem_LargeBody(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "text/plain")
	body := make([]byte, 5000) // > 4096 limit

	rce := &remotecall.RemoteCallError{
		Kind:         remotecall.ErrorKindHTTP,
		StatusCode:   500,
		ResponseBody: body,
		Headers:      headers,
	}

	problem, err := rce.ToProblem()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if problem.Detail != "" {
		t.Errorf("expected empty detail for large body, got %q", problem.Detail)
	}
}

func TestToProblem_ProblemJSONImplementsProblemInterface(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/problem+json")
	body := []byte(`{"type":"about:blank","title":"Error","status":500,"detail":"fail"}`)

	rce := &remotecall.RemoteCallError{
		Kind:         remotecall.ErrorKindHTTP,
		StatusCode:   500,
		ResponseBody: body,
		Headers:      headers,
	}

	problem, err := rce.ToProblem()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it implements the response.Problem interface
	var _ *response.Problem = problem
	if problem.HTTPStatus() != 500 {
		t.Errorf("expected HTTPStatus 500, got %d", problem.HTTPStatus())
	}
}
