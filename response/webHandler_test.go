package response

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/webFramework"
	"go.opentelemetry.io/otel/trace"
)

type captureParser struct {
	locals   map[string]any
	lastResp any
}

func (p *captureParser) GetMethod() string { return http.MethodPost }
func (p *captureParser) GetPath() string   { return "/" }
func (p *captureParser) GetHeader(webFramework.HeaderInterface) error {
	return nil
}
func (p *captureParser) GetHeaderValue(string) string                   { return "" }
func (p *captureParser) GetHttpHeader() http.Header                     { return nil }
func (p *captureParser) GetBody(any) error                              { return nil }
func (p *captureParser) GetUri(any) error                               { return nil }
func (p *captureParser) GetUrlQuery(any) error                          { return nil }
func (p *captureParser) GetRawUrlQuery() string                         { return "" }
func (p *captureParser) GetLocal(name string) any                       { return p.locals[name] }
func (p *captureParser) GetLocalString(name string) string              { return "" }
func (p *captureParser) GetUrlParam(string) string                      { return "" }
func (p *captureParser) GetUrlParams() map[string]string                { return nil }
func (p *captureParser) CheckUrlParam(string) (string, bool)            { return "", false }
func (p *captureParser) SetLocal(name string, value any)                { p.locals[name] = value }
func (p *captureParser) SetReqHeader(string, string)                    {}
func (p *captureParser) SetRespHeader(string, string)                   {}
func (p *captureParser) GetArgs(...any) map[string]string               { return nil }
func (p *captureParser) ParseCommand(string, string, webFramework.RecordData, webFramework.FieldParser) string {
	return ""
}
func (p *captureParser) SendJSONRespBody(_ int, resp any) error {
	p.lastResp = resp
	return nil
}
func (p *captureParser) Next() error                     { return nil }
func (p *captureParser) Abort() error                    { return nil }
func (p *captureParser) FormValue(string) string         { return "" }
func (p *captureParser) SaveFile(string, string) error   { return nil }
func (p *captureParser) FileAttachment(string, string)   {}
func (p *captureParser) AddCustomAttributes(slog.Attr)   {}
func (p *captureParser) GetTraceContext() trace.SpanContext {
	return trace.SpanContext{}
}
func (p *captureParser) SetTraceContext(trace.SpanContext) {}
func (p *captureParser) StartSpan(string, ...trace.SpanStartOption) (context.Context, trace.Span) {
	return context.Background(), nil
}
func (p *captureParser) AddSpanAttribute(string, string)          {}
func (p *captureParser) AddSpanAttributes(map[string]string)      {}
func (p *captureParser) AddSpanEvent(string, map[string]string)   {}
func (p *captureParser) RecordSpanError(error, map[string]string) {}
func (p *captureParser) GetContext() context.Context              { return context.Background() }
func (p *captureParser) SetContext(context.Context)               {}

func newTestWebFramework() (webFramework.WebFramework, *captureParser) {
	p := &captureParser{locals: make(map[string]any)}
	return webFramework.WebFramework{Parser: p}, p
}

func errorsFromResponse(t *testing.T, resp any) []ErrorResponse {
	t.Helper()
	ws, ok := resp.(WsResponse)
	if !ok {
		t.Fatalf("expected WsResponse, got %T", resp)
	}
	errs, ok := ws.ErrorData.([]ErrorResponse)
	if !ok {
		t.Fatalf("expected []ErrorResponse, got %T", ws.ErrorData)
	}
	return errs
}

func TestWebHandler_Error_ValidationFailedMultiField(t *testing.T) {
	handler := WebHanlder{}
	w, p := newTestWebFramework()

	validationErrs := []ErrorResponse{
		{Code: "REQUIRED-FIELD", Description: " فیلد رمز دوم کارت اجباری میباشد"},
		{Code: "REQUIRED-FIELD", Description: " فیلد شناسه cvv2 کارت اجباری میباشد"},
		{Code: "REQUIRED-FIELD", Description: " فیلد تاریخ انقضای کارت اجباری میباشد"},
	}
	err := errors.Join(libError.New(status.BadRequest, "VALIDATION_FAILED", validationErrs))

	handler.Error(w, err)

	errs := errorsFromResponse(t, p.lastResp)
	if len(errs) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(errs))
	}
	for i, want := range validationErrs {
		if errs[i].Code != want.Code || errs[i].Description != want.Description {
			t.Errorf("error[%d]: got (%q, %q), want (%q, %q)",
				i, errs[i].Code, errs[i].Description, want.Code, want.Description)
		}
	}
	for _, e := range errs {
		if e.Description == SYSTEM_FAULT_DESC {
			t.Errorf("unexpected system fault description: %+v", e)
		}
	}
}

func TestWebHandler_Error_PublicDescription(t *testing.T) {
	handler := WebHanlder{}
	w, p := newTestWebFramework()

	err := libError.ErrorData{
		ActionData: libError.Action{
			Status:            status.BadRequest,
			Description:       "MY_CODE",
			PublicDescription: "client visible text",
		},
	}

	handler.Error(w, err)

	errs := errorsFromResponse(t, p.lastResp)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != "MY_CODE" || errs[0].Description != "client visible text" {
		t.Errorf("got (%q, %q), want (MY_CODE, client visible text)", errs[0].Code, errs[0].Description)
	}
}

func TestWebHandler_Error_GenericErrorDescFallback(t *testing.T) {
	handler := WebHanlder{
		ErrorDesc: map[string]string{"SOME_CODE": "mapped description"},
	}
	w, p := newTestWebFramework()

	err := libError.New(status.InternalServerError, "SOME_CODE", "internal detail")

	handler.Error(w, err)

	errs := errorsFromResponse(t, p.lastResp)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Description != "mapped description" {
		t.Errorf("got description %q, want mapped description", errs[0].Description)
	}
}
