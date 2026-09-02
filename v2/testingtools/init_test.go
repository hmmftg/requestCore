package testingtools

import (
	"net/http"
	"testing"

	"github.com/hmmftg/requestCore/v2/request"
)

func TestNewTestExecutor(t *testing.T) {
	exec := NewTestExecutor()
	if exec == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestNewTestExecutorWithSink(t *testing.T) {
	exec := NewTestExecutorWithSink(nil)
	if exec == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestNewTestContext(t *testing.T) {
	ctx := NewTestContext()
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestNewTestTransport(t *testing.T) {
	tt := NewTestTransport()
	if tt == nil {
		t.Fatal("expected non-nil transport")
	}
	if tt.Committed() {
		t.Fatal("expected transport to not be committed initially")
	}
}

func TestTestTransport_WriteResponse(t *testing.T) {
	tt := NewTestTransport()
	err := tt.WriteResponse(http.StatusOK, "application/json", nil, []byte(`{}`))
	if err != nil {
		t.Fatalf("WriteResponse: %v", err)
	}
	if !tt.Committed() {
		t.Fatal("expected transport to be committed")
	}
	if tt.Status() != http.StatusOK {
		t.Fatalf("expected 200, got %d", tt.Status())
	}
	if tt.ContentType() != "application/json" {
		t.Fatalf("expected application/json, got %s", tt.ContentType())
	}
	if string(tt.Body()) != `{}` {
		t.Fatalf("expected body {}, got %s", string(tt.Body()))
	}
}

func TestTestTransport_MultiValueHeaders(t *testing.T) {
	tt := NewTestTransport()
	headers := http.Header{}
	headers.Add("Set-Cookie", "a=1")
	headers.Add("Set-Cookie", "b=2")
	err := tt.WriteResponse(http.StatusOK, "", headers, nil)
	if err != nil {
		t.Fatalf("WriteResponse: %v", err)
	}
	cookies := tt.Header()["Set-Cookie"]
	if len(cookies) != 2 {
		t.Fatalf("expected 2 Set-Cookie headers, got %d", len(cookies))
	}
}

func TestNewTestWorker(t *testing.T) {
	worker := NewTestWorker()
	if worker == nil {
		t.Fatal("expected non-nil worker")
	}
}

func TestAssertResponse(t *testing.T) {
	tt := NewTestTransport()
	_ = tt.WriteResponse(http.StatusOK, "application/json", nil, []byte(`{}`))
	if !AssertResponse(tt, http.StatusOK, "application/json") {
		t.Fatal("expected AssertResponse to return true")
	}
	if AssertResponse(tt, http.StatusNotFound, "application/json") {
		t.Fatal("expected AssertResponse to return false for wrong status")
	}
}

func TestDefaultRenderer(t *testing.T) {
	r := DefaultRenderer()
	if r == nil {
		t.Fatal("expected non-nil renderer")
	}
	if r.ContentType() != "application/json" {
		t.Fatalf("expected application/json, got %s", r.ContentType())
	}
}

func TestMappedErrorHandler(t *testing.T) {
	handler := MappedErrorHandler()
	if handler == nil {
		t.Fatal("expected non-nil error handler")
	}
	ctx := request.NewContext(nil)
	tt := NewTestTransport()
	handler(ctx, tt, nil) // nil error should be a no-op
	if tt.Committed() {
		t.Fatal("expected no response for nil error")
	}
}
