package webFramework

import (
	"context"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type RecordData interface {
	GetId() string
	GetControlId(string) string
	GetIdList() []any
	SetId(string)
	SetValue(string)
	GetSubCategory() string
	GetValue() any
	GetValueMap() map[string]string
}

type HeaderInterface interface {
	GetId() string
	GetUser() string
	GetProgram() string
	GetModule() string
	GetMethod() string
	SetUser(string)
	SetProgram(string)
	SetModule(string)
	SetMethod(string)
}

type FieldParser interface {
	Parse(string) string
}

type RequestParser interface {
	GetMethod() string
	GetPath() string
	GetHeader(target HeaderInterface) error
	GetHeaderValue(name string) string
	GetHttpHeader() http.Header
	GetBody(target any) error
	GetUri(target any) error
	GetUrlQuery(target any) error
	GetRawUrlQuery() string
	GetLocal(name string) any
	GetLocalString(name string) string
	GetUrlParam(name string) string
	GetUrlParams() map[string]string
	CheckUrlParam(name string) (string, bool)
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

type RequestHandler interface {
	Respond(code, status int, message string, data any, abort bool)
	HandleErrorState(err error, status int, message string, data any)
}

type WebFramework struct {
	Ctx     context.Context
	Span    trace.Span
	//Handler response.ResponseHandler
	Parser RequestParser
}

// Tracing methods for WebFramework
func (w *WebFramework) GetTraceContext() trace.SpanContext {
	if w.Span != nil {
		return w.Span.SpanContext()
	}
	return trace.SpanContext{}
}

func (w *WebFramework) SetSpan(span trace.Span) {
	w.Span = span
}

func (w *WebFramework) StartSpan(name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if w.Parser != nil {
		return w.Parser.StartSpan(name, opts...)
	}
	return w.Ctx, nil
}

func (w *WebFramework) AddSpanAttribute(key, value string) {
	if w.Span != nil && w.Span.IsRecording() {
		w.Span.SetAttributes(attribute.String(key, value))
	}
}

func (w *WebFramework) AddSpanAttributes(attrs map[string]string) {
	if w.Span != nil && w.Span.IsRecording() {
		for k, v := range attrs {
			w.Span.SetAttributes(attribute.String(k, v))
		}
	}
}

func (w *WebFramework) AddSpanEvent(name string, attrs map[string]string) {
	if w.Span != nil && w.Span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		w.Span.AddEvent(name, trace.WithAttributes(eventAttrs...))
	}
}

func (w *WebFramework) RecordSpanError(err error, attrs map[string]string) {
	if w.Span != nil && w.Span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		w.Span.RecordError(err, trace.WithAttributes(eventAttrs...))
	}
}
