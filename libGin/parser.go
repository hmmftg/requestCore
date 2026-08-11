// Package libGin provides a Gin web framework adapter for requestCore.
package libGin

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hmmftg/requestCore/libLogger"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/webFramework"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// InitContext creates a GinParser from a Gin context.
func InitContext(c any) GinParser {
	return GinParser{Ctx: c.(*gin.Context)}
}

// GetMethod returns the HTTP method of the request.
func (c GinParser) GetMethod() string {
	return c.Ctx.Request.Method
}

// GetPath returns the full route path of the request.
func (c GinParser) GetPath() string {
	return c.Ctx.FullPath()
}

// GetHeader binds request headers to the given target struct.
func (c GinParser) GetHeader(target webFramework.HeaderInterface) error {
	return c.Ctx.ShouldBindHeader(target)
}

// GetHeaderValue returns the value of the named HTTP request header.
func (c GinParser) GetHeaderValue(name string) string {
	return c.Ctx.Request.Header.Get(name)
}

// GetRawURLQuery returns the raw query string from the request URL.
func (c GinParser) GetRawURLQuery() string {
	return c.Ctx.Request.URL.RawQuery
}

// GetBody binds the JSON request body to the given target.
func (c GinParser) GetBody(target any) error {
	return c.Ctx.ShouldBindJSON(target)
}

// GetURI binds URI parameters to the given target.
func (c GinParser) GetURI(target any) error {
	return c.Ctx.ShouldBindUri(target)
}

// GetURLQuery binds URL query parameters to the given target.
func (c GinParser) GetURLQuery(target any) error {
	return c.Ctx.ShouldBindQuery(target)
}

// GetLocal returns the value stored in the Gin context under the given name.
func (c GinParser) GetLocal(name string) any {
	value, _ := c.Ctx.Get(name)
	return value
}

// GetLocalString returns the string value stored in the Gin context under the given name.
func (c GinParser) GetLocalString(name string) string {
	return c.Ctx.GetString(name)
}

// GetURLParam returns the value of the named URL path parameter.
func (c GinParser) GetURLParam(name string) string {
	return c.Ctx.Params.ByName(name)
}

// GetURLParams returns all URL path parameters as a map.
func (c GinParser) GetURLParams() map[string]string {
	ginParams := c.Ctx.Params
	result := make(map[string]string, 0)
	for _, param := range ginParams {
		result[param.Key] = param.Value
	}
	return result
}

// CheckURLParam returns the URL parameter value and whether it exists.
func (c GinParser) CheckURLParam(name string) (string, bool) {
	return c.Ctx.Params.Get(name)
}

// AddCustomAttributes adds a custom slog attribute to the request context.
func (c GinParser) AddCustomAttributes(attr slog.Attr) {
	/*
		idx := 0
		for id := range attrs {
			if attrs[id].Key == attr.Key {
				idx = 1
			}
			if attrs[id].Key == fmt.Sprintf("%s_%d", attr.Key, idx) {
				idx++
			}
		}
		if idx != 0 {
			attr.Key = fmt.Sprintf("%s_%d", attr.Key, idx)
			c.Set(customAttributesCtxKey, append(attrs, attr))
		} else {
			c.Set(customAttributesCtxKey, append(attrs, attr))
		}
	*/
	sloggin.AddCustomAttributes(c.Ctx, attr)
}

// SetLocal stores a value in the Gin context under the given name.
func (c GinParser) SetLocal(name string, value any) {
	c.Ctx.Set(name, value)
}

// SetReqHeader sets a value on the HTTP request header.
func (c GinParser) SetReqHeader(name string, value string) {
	c.Ctx.Request.Header.Set(name, value)
}

// SetRespHeader sets a value on the HTTP response header.
func (c GinParser) SetRespHeader(name string, value string) {
	c.Ctx.Header(name, value)
}

// GetArgs returns a map of common request arguments and additional URL parameters.
func (c GinParser) GetArgs(args ...any) map[string]string {
	ginArgs := map[string]string{
		"userId":   c.Ctx.GetString("userId"),
		"appName":  c.Ctx.GetString("appName"),
		"action":   c.Ctx.GetString("action"),
		"bankCode": c.Ctx.GetHeader("Bank-Code"),
	}

	for _, arg := range args {
		ginArgs[arg.(string)] = c.Ctx.Param(arg.(string))
	}

	return ginArgs
}

// ParseCommand parses a query command using request context values and field parsers.
func (c GinParser) ParseCommand(command, title string, request webFramework.RecordData, parser webFramework.FieldParser) string {
	return libQuery.ParseCommand(command,
		c.Ctx.GetString("userId"),
		c.Ctx.GetString("appName"),
		c.Ctx.GetString("action"),
		c.Ctx.GetString(title), request.GetValueMap(), parser)
}

// GetHTTPHeader returns the HTTP request headers.
func (c GinParser) GetHTTPHeader() http.Header {
	return c.Ctx.Request.Header
}

// SendJSONRespBody sends a JSON response with the given status code and body.
func (c GinParser) SendJSONRespBody(status int, resp any) error {
	c.SetLocal(libLogger.SlogResponseBody, resp)
	c.Ctx.JSON(status, resp)
	return nil
}

// Next calls the next handler in the Gin middleware chain.
func (c GinParser) Next() error {
	c.Ctx.Next()
	return nil
}

// Abort aborts the Gin middleware chain.
func (c GinParser) Abort() error {
	c.Ctx.Abort()
	return nil
}

// FormValue returns the value of the named form field.
func (c GinParser) FormValue(name string) string {
	value := c.Ctx.Request.FormValue(name)

	return value
}

// SaveFile saves an uploaded file from the form to the given path.
func (c GinParser) SaveFile(
	formTagName, path string,
) error {
	file, fileHeaders, fileErr := c.Ctx.Request.FormFile(formTagName)
	if fileErr != nil {
		return fileErr
	}
	defer func() { _ = file.Close() }()

	saveErr := c.Ctx.SaveUploadedFile(fileHeaders, path)
	if saveErr != nil {
		return saveErr
	}

	return nil
}

// FileAttachment sends a file as an attachment in the response.
func (c GinParser) FileAttachment(path, fileName string) {
	c.Ctx.FileAttachment(path, fileName)
}

// Gin adapts a requestCore handler into a Gin HandlerFunc.
func Gin(handler any) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler.(func(c context.Context))(c)
	}
}

// GetTraceContext returns the trace span context from the Gin request context.
func (c GinParser) GetTraceContext() trace.SpanContext {
	span := trace.SpanFromContext(c.Ctx.Request.Context())
	return span.SpanContext()
}

// SetTraceContext is a no-op for Gin as trace context is handled by the context.
func (c GinParser) SetTraceContext(_ trace.SpanContext) {
	// This is a no-op for Gin as trace context is handled by the context
}

// StartSpan returns the current span from the request context.
func (c GinParser) StartSpan(_ string, _ ...trace.SpanStartOption) (context.Context, trace.Span) {
	span := trace.SpanFromContext(c.Ctx.Request.Context())
	return c.Ctx.Request.Context(), span
}

// AddSpanAttribute adds a string key-value attribute to the current span.
func (c GinParser) AddSpanAttribute(key, value string) {
	span := trace.SpanFromContext(c.Ctx.Request.Context())
	if span.IsRecording() {
		span.SetAttributes(attribute.String(key, value))
	}
}

// AddSpanAttributes adds multiple string key-value attributes to the current span.
func (c GinParser) AddSpanAttributes(attrs map[string]string) {
	span := trace.SpanFromContext(c.Ctx.Request.Context())
	if span.IsRecording() {
		for k, v := range attrs {
			span.SetAttributes(attribute.String(k, v))
		}
	}
}

// AddSpanEvent records an event with attributes on the current span.
func (c GinParser) AddSpanEvent(name string, attrs map[string]string) {
	span := trace.SpanFromContext(c.Ctx.Request.Context())
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.AddEvent(name, trace.WithAttributes(eventAttrs...))
	}
}

// RecordSpanError records an error with attributes on the current span.
func (c GinParser) RecordSpanError(err error, attrs map[string]string) {
	span := trace.SpanFromContext(c.Ctx.Request.Context())
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.RecordError(err, trace.WithAttributes(eventAttrs...))
	}
}

// GetContext returns the context from the Gin request
func (c GinParser) GetContext() context.Context {
	return c.Ctx.Request.Context()
}

// SetContext updates the context in the Gin request
func (c GinParser) SetContext(ctx context.Context) {
	c.Ctx.Request = c.Ctx.Request.WithContext(ctx)
}
