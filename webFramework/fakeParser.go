package webFramework

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

// FakeParser is a test implementation of the RequestParser interface.
type FakeParser struct {
	Method      string
	Path        string
	Header      HeaderInterface
	HttpHeader  http.Header
	ReqHeader   map[string]string
	RespHeader  map[string]string
	Body        any
	URI         any
	URLQuery    any
	RawUrlQuery string
	Locals      map[string]any
	Args        map[string]string
	Urlparams   map[string]string
	JsonResp    any
}

// GetMethod returns the fake HTTP method.
func (f FakeParser) GetMethod() string {
	return f.Method
}

// GetPath returns the fake request path.
func (f FakeParser) GetPath() string {
	return f.Path
}

// GetHeader is a no-op that always returns nil for the fake parser.
func (f FakeParser) GetHeader(target HeaderInterface) error {
	return nil
}

// GetHeaderValue returns the fake request header value for the given name.
func (f FakeParser) GetHeaderValue(name string) string {
	return f.ReqHeader[name]
}

// GetHTTPHeader returns the fake HTTP headers.
func (f FakeParser) GetHTTPHeader() http.Header {
	return f.HttpHeader
}

// GetBody is a no-op that always returns nil for the fake parser.
func (f FakeParser) GetBody(target any) error {
	_ = target
	return nil
}

// GetURI is a no-op that always returns nil for the fake parser.
func (f FakeParser) GetURI(target any) error {
	_ = target
	return nil
}

// GetURLQuery is a no-op that always returns nil for the fake parser.
func (f FakeParser) GetURLQuery(target any) error {
	_ = target
	return nil
}

// GetRawURLQuery returns the fake raw URL query string.
func (f FakeParser) GetRawURLQuery() string {
	return f.RawUrlQuery
}

// GetLocal returns the fake local value for the given name.
func (f FakeParser) GetLocal(name string) any {
	return f.Locals[name]
}

// GetLocalString returns the fake local value as a string for the given name.
func (f FakeParser) GetLocalString(name string) string {
	return fmt.Sprintf("%v", f.Locals[name])
}

// GetURLParam returns the fake URL parameter value for the given name.
func (f FakeParser) GetURLParam(name string) string {
	return f.Urlparams[name]
}

// GetURLParams returns all fake URL parameters.
func (f FakeParser) GetURLParams() map[string]string {
	return f.Urlparams
}

// CheckURLParam returns the fake URL parameter and whether it exists.
func (f FakeParser) CheckURLParam(name string) (string, bool) {
	p, ok := f.Urlparams[name]
	return p, ok
}

// SetLocal stores a value in the fake parser's local storage.
func (f FakeParser) SetLocal(name string, value any) {
	f.Locals[name] = value
}

// GetLogger returns the slog logger from the fake parser's local storage.
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

// AddCustomAttributes adds a custom slog attribute to the fake parser's local storage.
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

// SetReqHeader sets a fake request header value.
func (f FakeParser) SetReqHeader(name string, value string) {
	f.ReqHeader[name] = value
}

// SetRespHeader sets a fake response header value.
func (f FakeParser) SetRespHeader(name string, value string) {
	f.RespHeader[name] = value
}

// GetArgs returns the fake request arguments map.
func (f FakeParser) GetArgs(args ...any) map[string]string {
	return f.Args
}

// ParseCommand returns an empty string for the fake parser.
func (f FakeParser) ParseCommand(command, title string, request RecordData, parser FieldParser) string {
	return ""
}

// SendJSONRespBody is a no-op that always returns nil for the fake parser.
func (f FakeParser) SendJSONRespBody(status int, resp any) error {
	_ = resp
	return nil
}

// Next is a no-op that always returns nil for the fake parser.
func (f FakeParser) Next() error {
	return nil
}

// Abort is a no-op that always returns nil for the fake parser.
func (f FakeParser) Abort() error {
	return nil
}

// FormValue returns the fake form value for the given name.
func (f FakeParser) FormValue(name string) string {
	return f.Args[name]
}

// SaveFile is a no-op that always returns nil for the fake parser.
func (f FakeParser) SaveFile(
	formTagName, path string,
) error {
	return nil
}

// FileAttachment is a no-op for the fake parser.
func (f FakeParser) FileAttachment(path, fileName string) {
	// no-op for fake parser
}

// Tracing methods for TestingParser
func (f FakeParser) GetTraceContext() trace.SpanContext {
	return trace.SpanContext{}
}

// SetTraceContext is a no-op for the fake parser.
func (f FakeParser) SetTraceContext(spanCtx trace.SpanContext) {
	// No-op for testing
}

// StartSpan returns a background context and span for the fake parser.
func (f FakeParser) StartSpan(name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return context.Background(), trace.SpanFromContext(context.Background())
}

// AddSpanAttribute is a no-op for the fake parser.
func (f FakeParser) AddSpanAttribute(key, value string) {
	// No-op for testing
}

// AddSpanAttributes is a no-op for the fake parser.
func (f FakeParser) AddSpanAttributes(attrs map[string]string) {
	// No-op for testing
}

// AddSpanEvent is a no-op for the fake parser.
func (f FakeParser) AddSpanEvent(name string, attrs map[string]string) {
	// No-op for testing
}

// RecordSpanError is a no-op for the fake parser.
func (f FakeParser) RecordSpanError(err error, attrs map[string]string) {
	// No-op for testing
}

// GetContext returns a background context for testing
func (f FakeParser) GetContext() context.Context {
	return context.Background()
}

// SetContext is a no-op for testing
func (f FakeParser) SetContext(ctx context.Context) {
	// No-op for testing
}
