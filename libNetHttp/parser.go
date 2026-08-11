// Package libNetHttp provides a net/http web framework adapter for requestCore.
package libNetHttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/webFramework"
)

// InitContext creates a new NetHttpParser from the given HTTP request and response writer.
func InitContext(r *http.Request, w http.ResponseWriter) *NetHttpParser {
	parser := &NetHttpParser{
		Request:  r,
		Response: w,
		Locals:   make(map[string]any),
		Params:   make(map[string]string),
	}
	for key, value := range URLParamsFromRequest(r) {
		parser.Params[key] = value
	}
	return parser
}

// GetMethod returns the HTTP method of the request.
func (c NetHttpParser) GetMethod() string {
	return c.Request.Method
}

// GetPath returns the URL path of the request.
func (c NetHttpParser) GetPath() string {
	return c.Request.URL.Path
}

// GetHeader populates the target struct with header values from the request.
func (c NetHttpParser) GetHeader(target webFramework.HeaderInterface) error {
	// Parse headers into target struct
	// This is a simplified implementation - you might want to use a more sophisticated header parsing
	// For now, we'll manually map common headers
	if target == nil {
		return nil
	}

	// Example implementation - you can expand this based on your HeaderInterface
	if user := c.Request.Header.Get("User-Id"); user != "" {
		target.SetUser(user)
	}
	if program := c.Request.Header.Get("Program"); program != "" {
		target.SetProgram(program)
	}
	if module := c.Request.Header.Get("Module"); module != "" {
		target.SetModule(module)
	}
	if method := c.Request.Header.Get("Method"); method != "" {
		target.SetMethod(method)
	}

	return nil
}

// GetHeaderValue returns the value of a single request header by name.
func (c NetHttpParser) GetHeaderValue(name string) string {
	return c.Request.Header.Get(name)
}

// GetHTTPHeader returns the full HTTP header map from the request.
func (c NetHttpParser) GetHTTPHeader() http.Header {
	return c.Request.Header
}

// GetBody reads and unmarshals the request body into the target.
func (c NetHttpParser) GetBody(target any) error {
	if c.Request.Body == nil {
		return nil
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}

	if len(body) == 0 {
		return nil
	}

	return json.Unmarshal(body, target)
}

// GetURI parses URL path parameters into the target struct.
func (c NetHttpParser) GetURI(target any) error {
	// Parse URL parameters into target struct
	// This is a simplified implementation
	if target == nil {
		return nil
	}

	// You might want to use a more sophisticated parameter parsing library
	// For now, we'll use reflection or manual mapping
	return c.parseStructFromMap(target, c.Params)
}

// GetURLQuery parses URL query parameters into the target struct.
func (c NetHttpParser) GetURLQuery(target any) error {
	if target == nil {
		return nil
	}

	queryParams := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	return c.parseStructFromMap(target, queryParams)
}

// GetRawURLQuery returns the raw query string from the request URL.
func (c NetHttpParser) GetRawURLQuery() string {
	return c.Request.URL.RawQuery
}

// GetLocal returns a value stored in the parser's local map by name.
func (c NetHttpParser) GetLocal(name string) any {
	return c.Locals[name]
}

// GetLocalString returns a string value stored in the parser's local map by name.
func (c NetHttpParser) GetLocalString(name string) string {
	if value, exists := c.Locals[name]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// GetURLParam returns a single URL path parameter by name.
func (c NetHttpParser) GetURLParam(name string) string {
	return c.Params[name]
}

// GetURLParams returns all URL path parameters as a map.
func (c NetHttpParser) GetURLParams() map[string]string {
	return c.Params
}

// CheckURLParam returns a URL path parameter by name and whether it exists.
func (c NetHttpParser) CheckURLParam(name string) (string, bool) {
	value, exists := c.Params[name]
	return value, exists
}

// SetLocal stores a value in the parser's local map by name.
func (c NetHttpParser) SetLocal(name string, value any) {
	c.Locals[name] = value
}

// SetReqHeader sets a request header value by name.
func (c NetHttpParser) SetReqHeader(name string, value string) {
	c.Request.Header.Set(name, value)
}

// SetRespHeader sets a response header value by name.
func (c NetHttpParser) SetRespHeader(name string, value string) {
	c.Response.Header().Set(name, value)
}

// GetArgs returns a map of common request arguments including user, app, action, and path.
func (c NetHttpParser) GetArgs(args ...any) map[string]string {
	netHttpArgs := map[string]string{
		"userId":   c.GetLocalString("userId"),
		"userName": c.GetLocalString("userName"),
		"appName":  c.GetLocalString("appName"),
		"action":   c.GetLocalString("action"),
		"bankCode": c.GetLocalString("bankCode"),
		"path":     c.Request.URL.Path,
	}

	for _, arg := range args {
		if argStr, ok := arg.(string); ok {
			netHttpArgs[argStr] = c.Params[argStr]
		}
	}

	return netHttpArgs
}

// ParseCommand parses a DML command template using local values and the provided request data.
func (c NetHttpParser) ParseCommand(command, title string, request webFramework.RecordData, parser webFramework.FieldParser) string {
	if request.GetValueMap() == nil {
		return libQuery.ParseCommand(command, c.GetLocalString("userId"),
			c.GetLocalString("appName"),
			c.GetLocalString("action"),
			title,
			map[string]string{}, parser)
	}
	return libQuery.ParseCommand(command, c.GetLocalString("userId"),
		c.GetLocalString("appName"),
		c.GetLocalString("action"),
		title,
		request.GetValueMap(), parser)
}

// SendJSONRespBody writes a JSON response with the given HTTP status code.
func (c NetHttpParser) SendJSONRespBody(status int, resp any) error {
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)

	if resp == nil {
		return nil
	}

	return json.NewEncoder(c.Response).Encode(resp)
}

// Next advances to the next middleware in the chain (no-op for net/http).
func (c NetHttpParser) Next() error {
	// For net/http, we don't have a built-in Next() concept like middleware chains
	// This could be implemented using a custom middleware pattern
	return nil
}

// Abort stops the middleware chain by writing an internal-server-error status.
func (c NetHttpParser) Abort() error {
	// For net/http, we can't really "abort" in the same way as Gin/Fiber
	// We can set a status and return early
	c.Response.WriteHeader(http.StatusInternalServerError)
	return nil
}

// FormValue returns the first form value for the given field name.
func (c NetHttpParser) FormValue(name string) string {
	return c.Request.FormValue(name)
}

// SaveFile saves an uploaded file from the given form tag to the specified path.
func (c NetHttpParser) SaveFile(formTagName, path string) error {
	// Clean the path to prevent directory traversal (gosec G304).
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) || containsTraversal(cleanPath) {
		return fmt.Errorf("invalid file path: %s", path)
	}
	file, _, err := c.Request.FormFile(formTagName)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Create the file
	dst, err := os.Create(cleanPath) // #nosec G304 -- path validated above
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()

	// Copy the uploaded file to the destination
	_, err = io.Copy(dst, file)
	if err != nil {
		return err
	}

	return nil
}

// containsTraversal checks whether the cleaned path still contains
// parent-directory references ("..") that could escape the intended directory.
func containsTraversal(cleanPath string) bool {
	return cleanPath == ".." || strings.HasPrefix(cleanPath, "../") || strings.HasPrefix(cleanPath, "..\\")
}

// FileAttachment sends a file as an HTTP attachment with the given filename.
func (c NetHttpParser) FileAttachment(path, fileName string) {
	c.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	http.ServeFile(c.Response, c.Request, path)
}

// AddCustomAttributes stores a custom slog attribute in the parser's local map.
func (c NetHttpParser) AddCustomAttributes(attr slog.Attr) {
	// For net/http, we can store custom attributes in locals
	// This is a simplified implementation
	if c.Locals == nil {
		c.Locals = make(map[string]any)
	}
	c.Locals[attr.Key] = attr.Value
}

// Helper function to parse map into struct
func (c NetHttpParser) parseStructFromMap(target any, data map[string]string) error {
	// This is a simplified implementation
	// You might want to use a more sophisticated library like mapstructure
	// or implement reflection-based parsing

	// For now, we'll use JSON marshaling/unmarshaling as a workaround
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonData, target)
}

// SetParams sets URL parameters (useful for routing)
func (c *NetHttpParser) SetParams(params map[string]string) {
	c.Params = params
}

// AddParam adds a single URL parameter
func (c *NetHttpParser) AddParam(key, value string) {
	if c.Params == nil {
		c.Params = make(map[string]string)
	}
	c.Params[key] = value
}

// ParseForm parses form data
func (c NetHttpParser) ParseForm() error {
	return c.Request.ParseForm()
}

// ParseMultipartForm parses multipart form data
// ParseMultipartForm parses multipart form data from the request with the given max memory.
func (c NetHttpParser) ParseMultipartForm(maxMemory int64) error {
	return c.Request.ParseMultipartForm(maxMemory)
}

// GetFormValue gets form value
func (c NetHttpParser) GetFormValue(key string) string {
	return c.Request.FormValue(key)
}

// GetFormValues gets all form values for a key
func (c NetHttpParser) GetFormValues(key string) []string {
	return c.Request.Form[key]
}

// GetPostFormValue gets POST form value
func (c NetHttpParser) GetPostFormValue(key string) string {
	return c.Request.PostFormValue(key)
}

// GetPostFormValues gets all POST form values for a key
func (c NetHttpParser) GetPostFormValues(key string) []string {
	return c.Request.PostForm[key]
}

// GetCookie gets a cookie by name
func (c NetHttpParser) GetCookie(name string) (*http.Cookie, error) {
	return c.Request.Cookie(name)
}

// GetCookies gets all cookies
func (c NetHttpParser) GetCookies() []*http.Cookie {
	return c.Request.Cookies()
}

// SetCookie sets a cookie
func (c NetHttpParser) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.Response, cookie)
}

// Redirect redirects to a URL
func (c NetHttpParser) Redirect(url string, statusCode int) {
	http.Redirect(c.Response, c.Request, url, statusCode)
}

// ServeFile serves a file
func (c NetHttpParser) ServeFile(name string) {
	http.ServeFile(c.Response, c.Request, name)
}

// ServeContent serves content
func (c NetHttpParser) ServeContent(name string, modtime time.Time, content io.ReadSeeker) {
	http.ServeContent(c.Response, c.Request, name, modtime, content)
}

// Tracing methods for NetHttpParser
func (c NetHttpParser) GetTraceContext() trace.SpanContext {
	span := trace.SpanFromContext(c.Request.Context())
	return span.SpanContext()
}

// SetTraceContext sets the trace span context on the request (no-op for net/http).
func (c NetHttpParser) SetTraceContext(spanCtx trace.SpanContext) {
	// This is a no-op for net/http as trace context is handled by the context
}

// StartSpan starts a new tracing span from the request context.
func (c NetHttpParser) StartSpan(name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	span := trace.SpanFromContext(c.Request.Context())
	return c.Request.Context(), span
}

// AddSpanAttribute adds a single string attribute to the current tracing span.
func (c NetHttpParser) AddSpanAttribute(key, value string) {
	span := trace.SpanFromContext(c.Request.Context())
	if span.IsRecording() {
		span.SetAttributes(attribute.String(key, value))
	}
}

// AddSpanAttributes adds multiple string attributes to the current tracing span.
func (c NetHttpParser) AddSpanAttributes(attrs map[string]string) {
	span := trace.SpanFromContext(c.Request.Context())
	if span.IsRecording() {
		for k, v := range attrs {
			span.SetAttributes(attribute.String(k, v))
		}
	}
}

// AddSpanEvent adds an event with attributes to the current tracing span.
func (c NetHttpParser) AddSpanEvent(name string, attrs map[string]string) {
	span := trace.SpanFromContext(c.Request.Context())
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.AddEvent(name, trace.WithAttributes(eventAttrs...))
	}
}

// RecordSpanError records an error with attributes on the current tracing span.
func (c NetHttpParser) RecordSpanError(err error, attrs map[string]string) {
	span := trace.SpanFromContext(c.Request.Context())
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.RecordError(err, trace.WithAttributes(eventAttrs...))
	}
}

// GetContext returns the context from the HTTP request
func (c NetHttpParser) GetContext() context.Context {
	return c.Request.Context()
}

// SetContext updates the context in the HTTP request.
// It uses a pointer receiver because http.Request.WithContext returns a
// new *http.Request and the mutation must be visible to callers.
func (c *NetHttpParser) SetContext(ctx context.Context) {
	c.Request = c.Request.WithContext(ctx)
}
