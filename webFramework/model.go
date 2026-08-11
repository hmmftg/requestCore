// Package webFramework provides the web framework abstraction layer for requestCore.
package webFramework

import (
	"context"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RecordData defines the interface for record data used in query and DML operations.
type RecordData interface {
	GetID() string
	GetControlID(string) string
	GetIDList() []any
	SetID(string)
	SetValue(string)
	GetSubCategory() string
	GetValue() any
	GetValueMap() map[string]string
}

// HeaderInterface defines the methods for accessing and mutating request headers.
type HeaderInterface interface {
	GetID() string
	GetUser() string
	GetProgram() string
	GetModule() string
	GetMethod() string
	SetUser(string)
	SetProgram(string)
	SetModule(string)
	SetMethod(string)
}

// FieldParser defines the interface for parsing field values in command templates.
type FieldParser interface {
	Parse(string) string
}

// RequestParser defines the interface for parsing HTTP requests in a web framework.
type RequestParser interface {
	GetMethod() string
	GetPath() string
	GetHeader(target HeaderInterface) error
	GetHeaderValue(name string) string
	GetHTTPHeader() http.Header
	GetBody(target any) error
	GetURI(target any) error
	GetURLQuery(target any) error
	GetRawURLQuery() string
	GetLocal(name string) any
	GetLocalString(name string) string
	GetURLParam(name string) string
	GetURLParams() map[string]string
	CheckURLParam(name string) (string, bool)
	SetLocal(name string, value any)
	SetReqHeader(name string, value string)
	SetRespHeader(name string, value string)
	GetArgs(args ...any) map[string]string
	ParseCommand(command, title string, request RecordData, parser FieldParser) string
	SendJSONRespBody(status int, resp any) error
	Next() error
	Abort() error
	FormValue(name string) string
	SaveFile(formTagName, path string) error
	FileAttachment(path, fileName string)
	AddCustomAttributes(attr slog.Attr)
	// Tracing methods
	GetTraceContext() trace.SpanContext
	SetTraceContext(spanCtx trace.SpanContext)
	StartSpan(name string, opts ...trace.SpanStartOption) (context.Context, trace.Span)
	AddSpanAttribute(key, value string)
	AddSpanAttributes(attrs map[string]string)
	AddSpanEvent(name string, attrs map[string]string)
	RecordSpanError(err error, attrs map[string]string)
	// Context management for tracing
	GetContext() context.Context
	SetContext(context.Context)
}

// RequestHandler defines the interface for responding to HTTP requests.
type RequestHandler interface {
	Respond(code, status int, message string, data any, abort bool)
	HandleErrorState(err error, status int, message string, data any)
}

// WebFramework holds the request context, tracing span, and parser for handling HTTP requests.
type WebFramework struct {
	Ctx  context.Context
	Span trace.Span
	//Handler response.ResponseHandler
	Parser RequestParser
}

// GetTraceContext returns the trace span context from the web framework span.
func (w *WebFramework) GetTraceContext() trace.SpanContext {
	if w.Span != nil {
		return w.Span.SpanContext()
	}
	return trace.SpanContext{}
}

// SetSpan sets the tracing span on the web framework.
func (w *WebFramework) SetSpan(span trace.Span) {
	w.Span = span
}

// StartSpan starts a new tracing span via the parser.
func (w *WebFramework) StartSpan(name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if w.Parser != nil {
		return w.Parser.StartSpan(name, opts...)
	}
	return w.Ctx, nil
}

// AddSpanAttribute adds a string key-value attribute to the web framework's span.
func (w *WebFramework) AddSpanAttribute(key, value string) {
	if w.Span != nil && w.Span.IsRecording() {
		w.Span.SetAttributes(attribute.String(key, value))
	}
}

// AddSpanAttributes adds multiple string key-value attributes to the web framework's span.
func (w *WebFramework) AddSpanAttributes(attrs map[string]string) {
	if w.Span != nil && w.Span.IsRecording() {
		for k, v := range attrs {
			w.Span.SetAttributes(attribute.String(k, v))
		}
	}
}

// AddSpanEvent records an event with attributes on the web framework's span.
func (w *WebFramework) AddSpanEvent(name string, attrs map[string]string) {
	if w.Span != nil && w.Span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		w.Span.AddEvent(name, trace.WithAttributes(eventAttrs...))
	}
}

// RecordSpanError records an error with attributes on the web framework's span.
func (w *WebFramework) RecordSpanError(err error, attrs map[string]string) {
	if w.Span != nil && w.Span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		w.Span.RecordError(err, trace.WithAttributes(eventAttrs...))
	}
}
