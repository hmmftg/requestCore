// nolint:,staticcheck,ineffassign
package webFramework

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

type FakeParser struct {
	Method      string
	Path        string
	Header      HeaderInterface
	HttpHeader  http.Header
	ReqHeader   map[string]string
	RespHeader  map[string]string
	Body        any
	Uri         any
	UrlQuery    any
	RawUrlQuery string
	Locals      map[string]any
	Args        map[string]string
	Urlparams   map[string]string
	JsonResp    any
}

func (f FakeParser) GetMethod() string {
	return f.Method
}
func (f FakeParser) GetPath() string {
	return f.Path
}
func (f FakeParser) GetHeader(target HeaderInterface) error {
	return nil
}
func (f FakeParser) GetHeaderValue(name string) string {
	return f.ReqHeader[name]
}
func (f FakeParser) GetHttpHeader() http.Header {
	return f.HttpHeader
}
func (f FakeParser) GetBody(target any) error {
	target = f.Body
	return nil
}
func (f FakeParser) GetUri(target any) error {
	target = f.Uri
	return nil
}
func (f FakeParser) GetUrlQuery(target any) error {
	target = f.UrlQuery
	return nil
}
func (f FakeParser) GetRawUrlQuery() string {
	return f.RawUrlQuery
}
func (f FakeParser) GetLocal(name string) any {
	return f.Locals[name]
}
func (f FakeParser) GetLocalString(name string) string {
	return fmt.Sprintf("%v", f.Locals[name])
}
func (f FakeParser) GetUrlParam(name string) string {
	return f.Urlparams[name]
}
func (f FakeParser) GetUrlParams() map[string]string {
	return f.Urlparams
}
func (f FakeParser) CheckUrlParam(name string) (string, bool) {
	p, ok := f.Urlparams[name]
	return p, ok
}
func (f FakeParser) SetLocal(name string, value any) {
	value = f.Locals[name]
}

func (f FakeParser) GetLogger() *slog.Logger {
	value, ok := f.Locals["logger"]
	if !ok {
		return nil
	}
	switch lg := value.(type) {
	case *slog.Logger:
		return lg
	}
	return nil
}

const (
	customAttributesCtxKey string = "slog-fake.custom-attributes"
)

func (t FakeParser) AddCustomAttributes(attr slog.Attr) {
	v, ok := t.Locals[customAttributesCtxKey]
	if !ok {
		t.Locals[customAttributesCtxKey] = []slog.Attr{attr}
		return
	}

	switch attrs := v.(type) {
	case []slog.Attr:
		t.Locals[customAttributesCtxKey] = append(attrs, attr)
	}
}
func (f FakeParser) SetReqHeader(name string, value string) {
	f.ReqHeader[name] = value
}
func (f FakeParser) SetRespHeader(name string, value string) {
	f.RespHeader[name] = value
}
func (f FakeParser) GetArgs(args ...any) map[string]string {
	return f.Args
}
func (f FakeParser) ParseCommand(command, title string, request RecordData, parser FieldParser) string {
	return ""
}
func (f FakeParser) SendJSONRespBody(status int, resp any) error {
	resp = f.JsonResp
	return nil
}
func (f FakeParser) Next() error {
	return nil
}
func (f FakeParser) Abort() error {
	return nil
}

func (c FakeParser) FormValue(name string) string {
	value := c.FormValue(name)

	return value
}

func (c FakeParser) SaveFile(
	formTagName, path string,
) error {
	fileErr := c.SaveFile(formTagName, path)
	if fileErr != nil {
		return fileErr
	}

	return nil
}

func (c FakeParser) FileAttachment(path, fileName string) {
	c.FileAttachment(path, fileName)
}

// Tracing methods for TestingParser
func (c FakeParser) GetTraceContext() trace.SpanContext {
	return trace.SpanContext{}
}

func (c FakeParser) SetTraceContext(spanCtx trace.SpanContext) {
	// No-op for testing
}

func (c FakeParser) StartSpan(name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return context.Background(), trace.SpanFromContext(context.Background())
}

func (c FakeParser) AddSpanAttribute(key, value string) {
	// No-op for testing
}

func (c FakeParser) AddSpanAttributes(attrs map[string]string) {
	// No-op for testing
}

func (c FakeParser) AddSpanEvent(name string, attrs map[string]string) {
	// No-op for testing
}

func (c FakeParser) RecordSpanError(err error, attrs map[string]string) {
	// No-op for testing
}
