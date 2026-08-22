package response

import (
	"context"
	"errors"
	"net/http"
	"testing"

	legacyError "github.com/hmmftg/requestCore/libError"
	legacyResponse "github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
	legacy "github.com/hmmftg/requestCore/webFramework"

	"github.com/hmmftg/requestCore/v2/renderers"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

func makeTestRequest() *v2wf.RequestContext {
	parser := v2wf.NewFakeParserV2()
	return &v2wf.RequestContext{
		Context:       context.Background(),
		LegacyContext: context.Background(),
		Parser:        parser,
		Legacy: legacy.WebFramework{
			Parser: parser,
		},
	}
}

func TestDefaultStatusResolver_NilError(t *testing.T) {
	if s := DefaultStatusResolver(nil); s != http.StatusOK {
		t.Fatalf("expected 200, got %d", s)
	}
}

func TestDefaultStatusResolver_LibError(t *testing.T) {
	err := legacyError.ErrorData{
		ActionData: legacyError.Action{
			Status:      status.StatusCode(http.StatusNotFound),
			Description: "NOT_FOUND",
		},
	}
	if s := DefaultStatusResolver(err); s != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", s)
	}
}

func TestDefaultStatusResolver_GenericError(t *testing.T) {
	if s := DefaultStatusResolver(errors.New("something")); s != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", s)
	}
}

func TestDefaultStatusResolver_HTTPStatuser(t *testing.T) {
	type customErr struct{ error }
	type withStatus struct {
		customErr
	}
	// Test with a type that implements HTTPStatus()
	err := httpStatuserErr{status: http.StatusTeapot}
	if s := DefaultStatusResolver(err); s != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", s)
	}
}

type httpStatuserErr struct {
	status int
}

func (e httpStatuserErr) Error() string   { return "teapot" }
func (e httpStatuserErr) HTTPStatus() int { return e.status }

func TestRegistry_RegisterAndHandle(t *testing.T) {
	reg := NewRegistry(nil)
	called := false
	if err := reg.Register(http.StatusNotFound, func(ctx v2wf.ErrorContext) error {
		called = true
		return EnsureJSONErrorResponse(ctx.Request, ctx.Status, "CUSTOM_404", "custom not found")
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Use a libError that resolves to 404
	err := legacyError.ErrorData{
		ActionData: legacyError.Action{
			Status:      status.StatusCode(http.StatusNotFound),
			Description: "NOT_FOUND",
		},
	}

	req := makeTestRequest()
	if err := reg.Handle(req, err); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !called {
		t.Fatal("expected custom handler to be called")
	}
	parser := req.Parser.(*v2wf.FakeParserV2)
	if parser.ResponseStatus != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", parser.ResponseStatus)
	}
}

func TestRegistry_Fallback(t *testing.T) {
	reg := NewRegistry(nil)
	fallbackCalled := false
	if err := reg.SetFallback(func(ctx v2wf.ErrorContext) error {
		fallbackCalled = true
		return nil
	}); err != nil {
		t.Fatalf("SetFallback: %v", err)
	}

	req := makeTestRequest()
	if err := reg.Handle(req, errors.New("unknown error")); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !fallbackCalled {
		t.Fatal("expected fallback to be called")
	}
}

func TestRegistry_Freeze(t *testing.T) {
	reg := NewRegistry(nil)
	reg.Freeze()
	if err := reg.Register(http.StatusNotFound, func(ctx v2wf.ErrorContext) error {
		return nil
	}); err == nil {
		t.Fatal("expected error when registering on frozen registry")
	}
	if err := reg.SetFallback(func(ctx v2wf.ErrorContext) error {
		return nil
	}); err == nil {
		t.Fatal("expected error when setting fallback on frozen registry")
	}
}

func TestRegistry_InvalidStatus(t *testing.T) {
	reg := NewRegistry(nil)
	if err := reg.Register(99, func(ctx v2wf.ErrorContext) error { return nil }); err == nil {
		t.Fatal("expected error for invalid status")
	}
	if err := reg.Register(600, func(ctx v2wf.ErrorContext) error { return nil }); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestRegistry_NilHandler(t *testing.T) {
	reg := NewRegistry(nil)
	if err := reg.Register(http.StatusNotFound, nil); err == nil {
		t.Fatal("expected error for nil handler")
	}
	if err := reg.SetFallback(nil); err == nil {
		t.Fatal("expected error for nil fallback")
	}
}

func TestRegistry_HandlerErrorTriggersFallback(t *testing.T) {
	reg := NewRegistry(nil)
	fallbackCalled := false
	if err := reg.Register(http.StatusNotFound, func(ctx v2wf.ErrorContext) error {
		return errors.New("handler failed")
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := reg.SetFallback(func(ctx v2wf.ErrorContext) error {
		fallbackCalled = true
		return nil
	}); err != nil {
		t.Fatalf("SetFallback: %v", err)
	}

	req := makeTestRequest()
	if err := reg.Handle(req, errors.New("not found")); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !fallbackCalled {
		t.Fatal("expected fallback to be called when handler fails")
	}
}

func TestDefaultErrorHandlers(t *testing.T) {
	handlers := DefaultErrorHandlers()
	if len(handlers) != 5 {
		t.Fatalf("expected 5 default handlers, got %d", len(handlers))
	}

	for status, handler := range handlers {
		req := makeTestRequest()
		ctx := v2wf.ErrorContext{
			Request: req,
			Error:   errors.New("test error"),
			Status:  status,
		}
		if err := handler(ctx); err != nil {
			t.Fatalf("handler for status %d failed: %v", status, err)
		}
		parser := req.Parser.(*v2wf.FakeParserV2)
		if !parser.ResponseWritten {
			t.Fatalf("expected response to be written for status %d", status)
		}
		if parser.ResponseStatus != status {
			t.Fatalf("expected status %d, got %d", status, parser.ResponseStatus)
		}
		if parser.ResponseContentType != "application/json" {
			t.Fatalf("expected application/json, got %s", parser.ResponseContentType)
		}
	}
}

func TestLegacyFallback(t *testing.T) {
	legacyHandler := legacyResponse.WebHanlder{
		MessageDesc: make(map[string]string),
		ErrorDesc:   make(map[string]string),
	}
	fallback := LegacyFallback(legacyHandler)

	req := makeTestRequest()
	ctx := v2wf.ErrorContext{
		Request: req,
		Error:   errors.New("legacy error"),
		Status:  http.StatusInternalServerError,
	}
	if err := fallback(ctx); err != nil {
		t.Fatalf("LegacyFallback: %v", err)
	}
}

func TestHandler_OK(t *testing.T) {
	reg := NewRegistry(nil)
	reg.SetFallback(LegacyFallback(legacyResponse.WebHanlder{
		MessageDesc: make(map[string]string),
		ErrorDesc:   make(map[string]string),
	}))
	h := NewHandler(reg, renderers.JSONRenderer{}, legacyResponse.WebHanlder{})

	req := makeTestRequest()
	if err := h.OK(req, map[string]string{"hello": "world"}); err != nil {
		t.Fatalf("OK: %v", err)
	}
	parser := req.Parser.(*v2wf.FakeParserV2)
	if parser.ResponseStatus != http.StatusOK {
		t.Fatalf("expected 200, got %d", parser.ResponseStatus)
	}
	if parser.ResponseContentType != "application/json" {
		t.Fatalf("expected application/json, got %s", parser.ResponseContentType)
	}
}

func TestHandler_OKWithRenderer(t *testing.T) {
	reg := NewRegistry(nil)
	h := NewHandler(reg, renderers.JSONRenderer{}, legacyResponse.WebHanlder{})

	req := makeTestRequest()
	if err := h.OKWithRenderer(req, renderers.TextRenderer{}, "hello"); err != nil {
		t.Fatalf("OKWithRenderer: %v", err)
	}
	parser := req.Parser.(*v2wf.FakeParserV2)
	if parser.ResponseContentType != "text/plain; charset=utf-8" {
		t.Fatalf("expected text/plain, got %s", parser.ResponseContentType)
	}
	if string(parser.ResponseBody) != "hello" {
		t.Fatalf("expected 'hello', got %s", string(parser.ResponseBody))
	}
}

func TestHandler_NoContent(t *testing.T) {
	reg := NewRegistry(nil)
	h := NewHandler(reg, renderers.JSONRenderer{}, legacyResponse.WebHanlder{})

	req := makeTestRequest()
	if err := h.NoContent(req); err != nil {
		t.Fatalf("NoContent: %v", err)
	}
	parser := req.Parser.(*v2wf.FakeParserV2)
	if parser.ResponseStatus != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", parser.ResponseStatus)
	}
}

func TestHandler_Error(t *testing.T) {
	reg := NewRegistry(nil)
	reg.SetFallback(func(ctx v2wf.ErrorContext) error {
		return EnsureJSONErrorResponse(ctx.Request, ctx.Status, "FALLBACK", "fallback error")
	})
	h := NewHandler(reg, renderers.JSONRenderer{}, legacyResponse.WebHanlder{})

	req := makeTestRequest()
	if err := h.Error(req, errors.New("test error")); err != nil {
		t.Fatalf("Error: %v", err)
	}
	parser := req.Parser.(*v2wf.FakeParserV2)
	if parser.ResponseStatus != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", parser.ResponseStatus)
	}
}

func TestHandler_DefaultRenderer(t *testing.T) {
	reg := NewRegistry(nil)
	h := NewHandler(reg, nil, legacyResponse.WebHanlder{})
	if h.DefaultRenderer().ContentType() != "application/json" {
		t.Fatal("expected default JSON renderer")
	}
}
