//revive:disable
package libContext

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore/webFramework"
)

// TestingParser implements the webFramework.RequestParser interface for testing.
type TestingParser struct {
	Root                     *testing.T
	Method, Path, RawQuery   string
	Header                   webFramework.HeaderInterface
	HeaderError              error
	Uri                      any
	UriError                 error
	HttpHeader               http.Header
	Body, UrlQuery           any
	BodyError, UrlQueryError error
	Headers                  *sync.Map
	Locals                   *sync.Map
	UrlParams                map[string]string
	Args                     map[string]string
	NextError                error
	AbortError               error
	SendError                error
	ParsedCommands           map[string]string
}

const (
	// HeaderEnvKey is the environment variable name for test headers.
	HeaderEnvKey = "h"
	// LocalEnvKey is the environment variable name for test locals.
	LocalEnvKey = "l"
)

func parseEnv(t *testing.T, key string) *sync.Map {
	rawEnv := os.Getenv(key)
	valueList := strings.Split(rawEnv, "@")
	result := sync.Map{}
	for _, h := range valueList {
		pair := strings.Split(h, "#")
		if len(pair) == 1 {
			t.Fatalf("bad environment: %s\n", pair)
		}
		result.Store(pair[0], pair[1])
	}
	return &result
}

func initTestContext(t *testing.T) TestingParser {
	headers := parseEnv(t, HeaderEnvKey)
	locals := parseEnv(t, LocalEnvKey)
	return TestingParser{
		Root:    t,
		Method:  os.Getenv("m"),
		Headers: headers,
		Locals:  locals,
	}
}

// GetMethod returns the HTTP method for the test request.
func (t TestingParser) GetMethod() string {
	return t.Method
}

// GetPath returns the URL path for the test request.
func (t TestingParser) GetPath() string {
	return t.Path
}

func setTarget(target any, value any) {
	targetPtr := reflect.ValueOf(target)
	targetPtr.Set(reflect.ValueOf(value))
}

// GetHeader populates the target struct with the test header value.
func (t TestingParser) GetHeader(target webFramework.HeaderInterface) error {
	setTarget(target, t.Header)
	return t.HeaderError
}

// GetHeaderValue returns a header value by name from the test headers map.
func (t TestingParser) GetHeaderValue(name string) string {
	storage, ok := t.Headers.Load(name)
	if !ok {
		return ""
	}
	head, ok := storage.(string)
	if !ok {
		t.Root.Fatalf("wrong header[%s] type:%T\n", name, storage)
	}

	return head
}

// GetHTTPHeader returns the full HTTP header map for the test request.
func (t TestingParser) GetHTTPHeader() http.Header {
	return t.HttpHeader
}

// GetBody populates the target with the test request body.
func (t TestingParser) GetBody(target any) error {
	setTarget(target, t.Body)
	return t.BodyError
}

// GetURI populates the target with the test URI parameters.
func (t TestingParser) GetURI(target any) error {
	setTarget(target, t.Uri)
	return t.UriError
}

// GetURLQuery populates the target with the test URL query parameters.
func (t TestingParser) GetURLQuery(target any) error {
	setTarget(target, t.UrlQuery)
	return t.UrlQueryError
}

// GetRawURLQuery returns the raw query string for the test request.
func (t TestingParser) GetRawURLQuery() string {
	return t.RawQuery
}

// GetLocal returns a value from the test locals map by name.
func (t TestingParser) GetLocal(name string) any {
	storage, ok := t.Locals.Load(name)
	if !ok {
		return nil
	}
	return storage
}

// GetLocalString returns a string value from the test locals map by name.
func (t TestingParser) GetLocalString(name string) string {
	storage := t.GetLocal(name)
	if storage == nil {
		return ""
	}
	loc, ok := storage.(string)
	if !ok {
		t.Root.Fatalf("wrong local[%s] type:%T\n", name, storage)
	}
	return loc
}

const (
	customAttributesCtxKey string = "slog-test.custom-attributes"
)

// AddCustomAttributes stores a custom slog attribute in the test locals map.
func (t TestingParser) AddCustomAttributes(attr slog.Attr) {
	v, ok := t.Locals.Load(customAttributesCtxKey)
	if !ok {
		t.Locals.Store(customAttributesCtxKey, []slog.Attr{attr})
		return
	}

	switch attrs := v.(type) {
	case []slog.Attr:
		attrs = append(attrs, attr)
		t.Locals.Store(customAttributesCtxKey, attrs)
	}
}

// GetURLParam returns a URL path parameter by name from the test parameters.
func (t TestingParser) GetURLParam(name string) string {
	return t.UrlParams[name]
}

// GetURLParams returns all URL path parameters for the test request.
func (t TestingParser) GetURLParams() map[string]string {
	return t.UrlParams
}

// CheckURLParam returns a URL path parameter by name and whether it exists.
func (t TestingParser) CheckURLParam(name string) (string, bool) {
	param, ok := t.UrlParams[name]
	return param, ok
}

// SetLocal stores a value in the test locals map by name.
func (t TestingParser) SetLocal(name string, value any) {
	t.Locals.Store(name, value)
}

// SetReqHeader sets a request header value in the test headers map.
func (t TestingParser) SetReqHeader(name string, value string) {
	t.Headers.Store(name, value)
}

// SetRespHeader sets a response header value in the test headers map.
func (t TestingParser) SetRespHeader(name string, value string) {
	t.Headers.Store(name, value)
}

// GetArgs returns the test arguments map.
func (t TestingParser) GetArgs(args ...any) map[string]string {
	return t.Args
}

// ParseCommand returns a pre-parsed command from the test parsed commands map.
func (t TestingParser) ParseCommand(command, title string, request webFramework.RecordData, parser webFramework.FieldParser) string {
	return t.ParsedCommands[command]
}

// SendJSONRespBody simulates sending a JSON response body and returns the configured send error.
func (t TestingParser) SendJSONRespBody(status int, resp any) error {
	return t.SendError
}

// Next simulates advancing to the next middleware and returns the configured next error.
func (t TestingParser) Next() error {
	return t.NextError
}

// Abort simulates aborting the middleware chain and returns the configured abort error.
func (t TestingParser) Abort() error {
	return t.AbortError
}

// FormValue returns an empty string for form values in the test parser.
func (c TestingParser) FormValue(name string) string {
	// value := c.FormValue(name)

	return ""
}

// SaveFile simulates saving an uploaded file and always returns nil in the test parser.
func (c TestingParser) SaveFile(
	formTagName, path string,
) error {
	// fileErr := c.SaveFile(formTagName, path)
	// if fileErr != nil {
	// 	return fileErr
	// }

	return nil
}

// FileAttachment simulates sending a file attachment (no-op in the test parser).
func (c TestingParser) FileAttachment(path, fileName string) {
	// c.FileAttachment(path, fileName)
}

// Tracing methods for TestingParser
func (t TestingParser) GetTraceContext() trace.SpanContext {
	return trace.SpanContext{}
}

// SetTraceContext sets the trace span context (no-op for testing).
func (t TestingParser) SetTraceContext(spanCtx trace.SpanContext) {
	// No-op for testing
}

// StartSpan starts a new tracing span and returns a background context with nil span for testing.
func (t TestingParser) StartSpan(name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return context.Background(), nil
}

// AddSpanAttribute adds a span attribute (no-op for testing).
func (t TestingParser) AddSpanAttribute(key, value string) {
	// No-op for testing
}

// AddSpanAttributes adds multiple span attributes (no-op for testing).
func (t TestingParser) AddSpanAttributes(attrs map[string]string) {
	// No-op for testing
}

// AddSpanEvent adds a span event (no-op for testing).
func (t TestingParser) AddSpanEvent(name string, attrs map[string]string) {
	// No-op for testing
}

// RecordSpanError records a span error (no-op for testing).
func (t TestingParser) RecordSpanError(err error, attrs map[string]string) {
	// No-op for testing
}

// GetContext returns a background context for testing
func (t TestingParser) GetContext() context.Context {
	return context.Background()
}

// SetContext is a no-op for testing
func (t TestingParser) SetContext(ctx context.Context) {
	// No-op for testing
}
