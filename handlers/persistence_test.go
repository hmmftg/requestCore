package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"testing"

	"github.com/hmmftg/requestCore/webFramework"
	"go.opentelemetry.io/otel/trace"
)

type mapParser struct {
	locals map[string]any
}

func (p *mapParser) GetMethod() string { return http.MethodPost }
func (p *mapParser) GetPath() string   { return "/" }
func (p *mapParser) GetHeader(webFramework.HeaderInterface) error {
	return nil
}
func (p *mapParser) GetHeaderValue(string) string                   { return "" }
func (p *mapParser) GetHttpHeader() http.Header                     { return nil }
func (p *mapParser) GetBody(any) error                              { return nil }
func (p *mapParser) GetUri(any) error                               { return nil }
func (p *mapParser) GetUrlQuery(any) error                          { return nil }
func (p *mapParser) GetRawUrlQuery() string                         { return "" }
func (p *mapParser) GetLocal(name string) any                       { return p.locals[name] }
func (p *mapParser) GetLocalString(name string) string              { return "" }
func (p *mapParser) GetUrlParam(string) string                      { return "" }
func (p *mapParser) GetUrlParams() map[string]string                { return nil }
func (p *mapParser) CheckUrlParam(string) (string, bool)            { return "", false }
func (p *mapParser) SetLocal(name string, value any)                { p.locals[name] = value }
func (p *mapParser) SetReqHeader(string, string)                    {}
func (p *mapParser) SetRespHeader(string, string)                   {}
func (p *mapParser) GetArgs(...any) map[string]string               { return nil }
func (p *mapParser) ParseCommand(string, string, webFramework.RecordData, webFramework.FieldParser) string {
	return ""
}
func (p *mapParser) SendJSONRespBody(int, any) error { return nil }
func (p *mapParser) Next() error                     { return nil }
func (p *mapParser) Abort() error                    { return nil }
func (p *mapParser) FormValue(string) string         { return "" }
func (p *mapParser) SaveFile(string, string) error   { return nil }
func (p *mapParser) FileAttachment(string, string)   {}
func (p *mapParser) AddCustomAttributes(slog.Attr)   {}
func (p *mapParser) GetTraceContext() trace.SpanContext {
	return trace.SpanContext{}
}
func (p *mapParser) SetTraceContext(trace.SpanContext) {}
func (p *mapParser) StartSpan(string, ...trace.SpanStartOption) (context.Context, trace.Span) {
	return context.Background(), nil
}
func (p *mapParser) AddSpanAttribute(string, string)          {}
func (p *mapParser) AddSpanAttributes(map[string]string)      {}
func (p *mapParser) AddSpanEvent(string, map[string]string)   {}
func (p *mapParser) RecordSpanError(error, map[string]string) {}
func (p *mapParser) GetContext() context.Context              { return context.Background() }
func (p *mapParser) SetContext(context.Context)               {}

func TestPersistedRecordIDHelpers(t *testing.T) {
	w := webFramework.WebFramework{
		Parser: &mapParser{locals: make(map[string]any)},
	}

	SetPersistedRecordID(w, int64(42))
	id, ok := GetPersistedRecordID(w)
	if !ok {
		t.Fatal("expected persisted record id to be present")
	}
	if id.(int64) != 42 {
		t.Fatalf("expected id 42, got %v", id)
	}

	SetPersistedRecordID(w, "uuid-abc")
	id, ok = GetPersistedRecordID(w)
	if !ok || id.(string) != "uuid-abc" {
		t.Fatalf("expected uuid-abc, got %v ok=%v", id, ok)
	}
}

func TestFuncPersister(t *testing.T) {
	var insertCalled, updateCalled bool
	p := FuncPersister[testReq, testResp]{
		InsertFn: func(path string, req *HandlerRequest[testReq, testResp]) error {
			insertCalled = true
			if path != "/path" {
				t.Fatalf("unexpected path %q", path)
			}
			return nil
		},
		UpdateFn: func(path string, req *HandlerRequest[testReq, testResp]) error {
			updateCalled = true
			return nil
		},
	}

	trx := &HandlerRequest[testReq, testResp]{}
	if err := p.Insert("/path", trx); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := p.Update("/path", trx); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !insertCalled || !updateCalled {
		t.Fatal("expected Insert and Update fns to be called")
	}

	nop := FuncPersister[testReq, testResp]{}
	if err := nop.Insert("/path", trx); err != nil {
		t.Fatalf("nil InsertFn: %v", err)
	}
	if err := nop.Update("/path", trx); err != nil {
		t.Fatalf("nil UpdateFn: %v", err)
	}
}
