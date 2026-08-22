package testingtools

import (
	"context"
	"net/http"
	"testing"
)

func TestNewTestParserV2(t *testing.T) {
	p := NewTestParserV2()
	if p == nil {
		t.Fatal("expected non-nil parser")
	}
	if p.Cookies == nil {
		t.Fatal("expected initialized cookies map")
	}
}

func TestTestParserV2_SendResponse(t *testing.T) {
	p := NewTestParserV2()
	err := p.SendResponse(http.StatusOK, "application/json", []byte(`{}`))
	if err != nil {
		t.Fatalf("SendResponse: %v", err)
	}
	if !p.ResponseWritten {
		t.Fatal("expected response to be written")
	}
	if p.ResponseStatus != http.StatusOK {
		t.Fatalf("expected 200, got %d", p.ResponseStatus)
	}
}

func TestTestParserV2_SendResponseError(t *testing.T) {
	p := NewTestParserV2()
	p.SendResponseError = http.ErrAbortHandler
	err := p.SendResponse(http.StatusOK, "application/json", nil)
	if err != http.ErrAbortHandler {
		t.Fatalf("expected custom error, got %v", err)
	}
	if p.ResponseWritten {
		t.Fatal("expected response not to be written when error is returned")
	}
}

func TestTestRequestContext(t *testing.T) {
	ctx := TestRequestContext()
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.Parser == nil {
		t.Fatal("expected non-nil parser")
	}
	if ctx.Legacy.Parser == nil {
		t.Fatal("expected non-nil legacy parser")
	}
}

func TestTestRequestContextWithSession(t *testing.T) {
	ctx := TestRequestContextWithSession()
	if ctx.Session == nil {
		t.Fatal("expected non-nil session")
	}
	if ctx.Flash == nil {
		t.Fatal("expected non-nil flash")
	}
}

func TestInitTestingV2(t *testing.T) {
	handler, registry, renderer := InitTestingV2()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if renderer == nil {
		t.Fatal("expected non-nil renderer")
	}
	if renderer.ContentType() != "application/json" {
		t.Fatalf("expected JSON renderer, got %s", renderer.ContentType())
	}
}

func TestInitTestingV2WithWorker(t *testing.T) {
	handler, registry, renderer, worker := InitTestingV2WithWorker()
	if handler == nil || registry == nil || renderer == nil {
		t.Fatal("expected non-nil components")
	}
	if worker == nil {
		t.Fatal("expected non-nil worker")
	}
	defer func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_ = worker.Shutdown(ctx)
	}()
}

func TestAssertResponse(t *testing.T) {
	ctx := TestRequestContext()
	parser := ctx.Parser.(*TestParserV2)
	parser.SendResponse(http.StatusOK, "application/json", []byte(`{}`))
	if !AssertResponse(parser.FakeParserV2, http.StatusOK, "application/json") {
		t.Fatal("expected AssertResponse to return true")
	}
	if AssertResponse(parser.FakeParserV2, http.StatusNotFound, "application/json") {
		t.Fatal("expected AssertResponse to return false for wrong status")
	}
}

func TestSetRequestCookie(t *testing.T) {
	ctx := TestRequestContext()
	SetRequestCookie(ctx, "test", "value")
	parser := ctx.Parser.(*TestParserV2)
	if parser.Cookies["test"] != "value" {
		t.Fatalf("expected cookie 'test'='value', got %q", parser.Cookies["test"])
	}
}

func TestGetSetCookies(t *testing.T) {
	ctx := TestRequestContext()
	parser := ctx.Parser.(*TestParserV2)
	parser.SetCookie(&http.Cookie{Name: "session", Value: "abc"})
	cookies := GetSetCookies(ctx)
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != "session" {
		t.Fatalf("expected 'session', got %s", cookies[0].Name)
	}
}
