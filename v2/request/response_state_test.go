package request

import (
	"net/http"
	"testing"
)

func TestResponseState_Defaults(t *testing.T) {
	r := NewResponseState()
	if r.Status() != 200 {
		t.Fatalf("expected default status 200, got %d", r.Status())
	}
	h := r.Header()
	if len(h) != 0 {
		t.Fatalf("expected empty headers, got %v", h)
	}
}

func TestResponseState_SetStatus(t *testing.T) {
	r := NewResponseState()
	r.SetStatus(201)
	if r.Status() != 201 {
		t.Fatalf("expected 201, got %d", r.Status())
	}
	if !r.StatusSet() {
		t.Fatalf("expected StatusSet true after SetStatus")
	}
}

func TestResponseState_DefaultStatusNotSet(t *testing.T) {
	r := NewResponseState()
	if r.StatusSet() {
		t.Fatalf("expected StatusSet false for default status")
	}
	if r.Status() != 200 {
		t.Fatalf("expected default status 200, got %d", r.Status())
	}
}

func TestResponseState_ClonePreservesStatusSet(t *testing.T) {
	r := NewResponseState()
	r.SetStatus(203)
	clone := r.Clone()
	if !clone.StatusSet() {
		t.Fatalf("expected cloned StatusSet true")
	}
	if clone.Status() != 203 {
		t.Fatalf("expected cloned status 203, got %d", clone.Status())
	}
	// Default clone should preserve unset flag.
	defClone := NewResponseState().Clone()
	if defClone.StatusSet() {
		t.Fatalf("expected cloned default StatusSet false")
	}
}

func TestResponseState_SetHeader(t *testing.T) {
	r := NewResponseState()
	r.SetHeader("Location", "/users/1")
	h := r.Header()
	if h.Get("Location") != "/users/1" {
		t.Fatalf("expected Location /users/1, got %q", h.Get("Location"))
	}
}

func TestResponseState_AddHeader(t *testing.T) {
	r := NewResponseState()
	r.AddHeader("X-Custom", "a")
	r.AddHeader("X-Custom", "b")
	h := r.Header()
	values := h["X-Custom"]
	if len(values) != 2 || values[0] != "a" || values[1] != "b" {
		t.Fatalf("expected [a, b], got %v", values)
	}
}

func TestResponseState_SetHeaderReplaces(t *testing.T) {
	r := NewResponseState()
	r.AddHeader("X-Custom", "a")
	r.SetHeader("X-Custom", "b")
	h := r.Header()
	values := h["X-Custom"]
	if len(values) != 1 || values[0] != "b" {
		t.Fatalf("expected [b], got %v", values)
	}
}

func TestResponseState_HeaderReturnsCopy(t *testing.T) {
	r := NewResponseState()
	r.SetHeader("X-Test", "value")
	h := r.Header()
	h.Set("X-Test", "mutated")
	// Original should be unaffected.
	if r.Header().Get("X-Test") != "value" {
		t.Fatalf("expected original value 'value', got %q", r.Header().Get("X-Test"))
	}
}

func TestResponseState_Clone(t *testing.T) {
	r := NewResponseState()
	r.SetStatus(204)
	r.SetHeader("X-Custom", "test")
	r.AddHeader("X-Multi", "a")
	r.AddHeader("X-Multi", "b")

	clone := r.Clone()
	if clone.Status() != 204 {
		t.Fatalf("expected cloned status 204, got %d", clone.Status())
	}
	ch := clone.Header()
	if ch.Get("X-Custom") != "test" {
		t.Fatalf("expected cloned X-Custom 'test', got %q", ch.Get("X-Custom"))
	}
	multi := ch["X-Multi"]
	if len(multi) != 2 || multi[0] != "a" || multi[1] != "b" {
		t.Fatalf("expected cloned X-Multi [a, b], got %v", multi)
	}
	// Mutating clone should not affect original.
	clone.SetHeader("X-Custom", "mutated")
	if r.Header().Get("X-Custom") != "test" {
		t.Fatalf("expected original unchanged, got %q", r.Header().Get("X-Custom"))
	}
}

func TestResponseState_ConcurrentAccess(t *testing.T) {
	r := NewResponseState()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			r.SetStatus(200 + i%10)
		}
	}()
	for i := 0; i < 100; i++ {
		_ = r.Status()
		_ = r.Header()
	}
	<-done
}

func TestResponseState_EmptyHeaderMap(t *testing.T) {
	r := NewResponseState()
	h := r.Header()
	if h == nil {
		t.Fatal("expected non-nil header map")
	}
	// Verify it's a valid http.Header that can be used.
	h.Set("X-Test", "value")
	if h.Get("X-Test") != "value" {
		t.Fatal("expected to be able to set on returned header map")
	}
	// Original should still be empty.
	if r.Header().Get("X-Test") != "" {
		t.Fatal("expected original to remain empty")
	}
}

func TestResponseState_HeadersWithHttpHeaderType(t *testing.T) {
	r := NewResponseState()
	r.SetHeader("Content-Type", "application/json")
	r.SetHeader("Accept", "text/plain")
	h := r.Header()
	if h.Get("Content-Type") != "application/json" {
		t.Fatalf("expected application/json, got %q", h.Get("Content-Type"))
	}
	if h.Get("Accept") != "text/plain" {
		t.Fatalf("expected text/plain, got %q", h.Get("Accept"))
	}
	// Verify it's a proper http.Header.
	if _, ok := any(h).(http.Header); !ok {
		t.Fatal("expected returned header to be http.Header")
	}
}
