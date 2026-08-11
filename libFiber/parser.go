// Package libFiber provides a Fiber web framework adapter for requestCore.
package libFiber

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/webFramework"

	"github.com/gofiber/fiber/v2"
	slogfiber "github.com/samber/slog-fiber"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// InitContext creates a FiberParser from the given Fiber context.
func InitContext(c *fiber.Ctx) FiberParser {
	return FiberParser{
		Ctx: c,
	}
}

// GetMethod returns the HTTP method of the Fiber request.
func (c FiberParser) GetMethod() string {
	return c.Ctx.Method()
}

// GetPath returns the original URL path of the Fiber request.
func (c FiberParser) GetPath() string {
	return c.Ctx.OriginalURL()
}

// GetHeader parses request headers into the target struct.
func (c FiberParser) GetHeader(target webFramework.HeaderInterface) error {
	targetPtr := target
	return c.Ctx.ReqHeaderParser(targetPtr)
}

// GetHeaderValue returns a single request header value by name.
func (c FiberParser) GetHeaderValue(name string) string {
	if len(c.Ctx.GetReqHeaders()[name]) > 0 {
		return c.Ctx.GetReqHeaders()[name][0]
	}
	return ""
}

// GetBody parses the request body into the target struct.
func (c FiberParser) GetBody(target any) error {
	return c.Ctx.BodyParser(target)
}

// GetURI parses URL path parameters into the target struct.
func (c FiberParser) GetURI(target any) error {
	return c.Ctx.ParamsParser(target)
}

// GetURLQuery parses URL query parameters into the target struct.
func (c FiberParser) GetURLQuery(target any) error {
	return c.Ctx.BodyParser(target)
}

// GetRawURLQuery returns the raw query string from the Fiber request.
func (c FiberParser) GetRawURLQuery() string {
	return string(c.Ctx.Request().URI().QueryString())
}

// GetLocal returns a value stored in Fiber locals by name.
func (c FiberParser) GetLocal(name string) any {
	return c.Ctx.Locals(name)
}

// GetLocalString returns a string value stored in Fiber locals by name.
func (c FiberParser) GetLocalString(name string) string {
	value := c.Ctx.Locals(name)
	switch str := value.(type) {
	case string:
		return str
	}
	return ""
}

// AddCustomAttributes adds a custom slog attribute to the Fiber context.
func (c FiberParser) AddCustomAttributes(attr slog.Attr) {
	slogfiber.AddCustomAttributes(c.Ctx, attr)
}

// GetURLParam returns a single URL path parameter by name.
func (c FiberParser) GetURLParam(name string) string {
	return c.Ctx.Params(name)
}

// GetURLParams returns all URL path parameters as a map.
func (c FiberParser) GetURLParams() map[string]string {
	return c.Ctx.AllParams()
}

// CheckURLParam returns a URL path parameter by name and whether it exists.
func (c FiberParser) CheckURLParam(name string) (string, bool) {
	value := c.Ctx.Params(name)
	return value, len(value) > 0
}

// SetLocal stores a value in Fiber locals by name.
func (c FiberParser) SetLocal(name string, value any) {
	c.Ctx.Locals(name, value)
}

// SetReqHeader sets a request header value by name.
func (c FiberParser) SetReqHeader(name string, value string) {
	c.Ctx.Context().Request.Header.Set(name, value)
}

// SetRespHeader sets a response header value by name.
func (c FiberParser) SetRespHeader(name string, value string) {
	c.Ctx.Context().Response.Header.Set(name, value)
}

// GetArgs returns a map of common request arguments including user, app, action, and path.
func (c FiberParser) GetArgs(args ...any) map[string]string {
	fiberArgs := map[string]string{
		"userId":   c.Ctx.Locals("userId").(string),
		"userName": c.Ctx.Locals("userName").(string),
		"appName":  c.Ctx.Locals("appName").(string),
		"action":   c.Ctx.Locals("action").(string),
		"bankCode": c.Ctx.Locals("bankCode").(string),
		"path":     c.Ctx.Route().Path,
	}

	for _, arg := range args {
		fiberArgs[arg.(string)] = c.Ctx.Params(arg.(string))
	}

	return fiberArgs
}

// ParseCommand parses a DML command template using local values and the provided request data.
func (c FiberParser) ParseCommand(command, title string, request webFramework.RecordData, parser webFramework.FieldParser) string {
	if request.GetValueMap() == nil {
		return libQuery.ParseCommand(command, c.Ctx.Locals("userId").(string),
			c.Ctx.Locals("appName").(string),
			c.Ctx.Locals("action").(string),
			title,
			map[string]string{}, parser)
	}
	return libQuery.ParseCommand(command, c.Ctx.Locals("userId").(string),
		c.Ctx.Locals("appName").(string),
		c.Ctx.Locals("action").(string),
		title,
		request.GetValueMap(), parser)
}

// GetHTTPHeader returns the full HTTP header map from the Fiber request.
func (c FiberParser) GetHTTPHeader() http.Header {
	return c.Ctx.GetReqHeaders()
}

// SendJSONRespBody writes a JSON response with the given HTTP status code.
func (c FiberParser) SendJSONRespBody(status int, resp any) error {
	err := c.Ctx.JSON(resp)
	c.Ctx.Status(status)
	return err
}

// Next advances to the next middleware in the Fiber chain.
func (c FiberParser) Next() error {
	return c.Ctx.Next()
}

// Abort stops the middleware chain by sending the current response status.
func (c FiberParser) Abort() error {
	return c.Ctx.SendStatus(c.Ctx.Response().StatusCode())
}

// FormValue returns the form value for the given field name.
func (c FiberParser) FormValue(name string) string {
	value := c.Ctx.FormValue(name, "")

	return value
}

// SaveFile saves an uploaded file from the given form tag to the specified path.
func (c FiberParser) SaveFile(
	formTagName, path string,
) error {
	fileHeader, fileErr := c.Ctx.FormFile(formTagName)
	if fileErr != nil {
		return fileErr
	}

	saveErr := c.Ctx.SaveFile(fileHeader, path)
	if saveErr != nil {
		return saveErr
	}

	return nil
}

// FileAttachment sends a file as an HTTP attachment with the given filename.
func (c FiberParser) FileAttachment(path, fileName string) {
	file := fmt.Sprintf("%s%s", path, fileName)
	_ = c.Ctx.SendFile(file, true)
}

// FiberCtxKey is the user-value key used to store the Fiber context in fasthttp.
const FiberCtxKey = "fiber.Ctx"

// Fiber wraps a handler function into a Fiber handler that stores the context and invokes the handler.
func Fiber(handler any) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		c.Context().SetUserValue(FiberCtxKey, c)
		handler.(func(c context.Context))(c.Context())
		return nil
	}
}

// ExtendMap converts a map[string]string to a map[string][]string for header compatibility.
func ExtendMap(mp map[string]string) map[string][]string {
	newMap := map[string][]string{}
	for key, val := range mp {
		newMap[key] = []string{val}
	}
	return newMap
}

// GetTraceContext returns the trace span context from the Fiber user context.
func (c FiberParser) GetTraceContext() trace.SpanContext {
	span := trace.SpanFromContext(c.Ctx.UserContext())
	return span.SpanContext()
}

// SetTraceContext sets the trace span context (no-op for Fiber).
func (c FiberParser) SetTraceContext(_ trace.SpanContext) {
	// This is a no-op for Fiber as trace context is handled by the context
}

// StartSpan starts a new tracing span from the Fiber user context.
func (c FiberParser) StartSpan(_ string, _ ...trace.SpanStartOption) (context.Context, trace.Span) {
	span := trace.SpanFromContext(c.Ctx.UserContext())
	return c.Ctx.UserContext(), span
}

// AddSpanAttribute adds a single string attribute to the current tracing span.
func (c FiberParser) AddSpanAttribute(key, value string) {
	span := trace.SpanFromContext(c.Ctx.UserContext())
	if span.IsRecording() {
		span.SetAttributes(attribute.String(key, value))
	}
}

// AddSpanAttributes adds multiple string attributes to the current tracing span.
func (c FiberParser) AddSpanAttributes(attrs map[string]string) {
	span := trace.SpanFromContext(c.Ctx.UserContext())
	if span.IsRecording() {
		for k, v := range attrs {
			span.SetAttributes(attribute.String(k, v))
		}
	}
}

// AddSpanEvent adds an event with attributes to the current tracing span.
func (c FiberParser) AddSpanEvent(name string, attrs map[string]string) {
	span := trace.SpanFromContext(c.Ctx.UserContext())
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.AddEvent(name, trace.WithAttributes(eventAttrs...))
	}
}

// RecordSpanError records an error with attributes on the current tracing span.
func (c FiberParser) RecordSpanError(err error, attrs map[string]string) {
	span := trace.SpanFromContext(c.Ctx.UserContext())
	if span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		span.RecordError(err, trace.WithAttributes(eventAttrs...))
	}
}

// GetContext returns the context from the Fiber context
func (c FiberParser) GetContext() context.Context {
	return c.Ctx.UserContext()
}

// SetContext updates the context in the Fiber context
func (c FiberParser) SetContext(ctx context.Context) {
	c.Ctx.SetUserContext(ctx)
}
