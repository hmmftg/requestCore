package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/libTracing"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type HandlerParameters struct {
	Title           string
	Body            libRequest.Type
	ValidateHeader  bool
	SaveToRequest   bool
	Path            string
	HasReceipt      bool
	RecoveryHandler func(any)
	FileResponse    bool
	LogArrays       []string
	LogTags         []string
	// Tracing parameters
	EnableTracing   bool
	TracingSpanName string
}

type HandlerInterface[Req any, Resp any] interface {
	// returns handler title
	//   Request Bodymode
	//   and validate header option
	//   and save to request table option
	//   and url path of handler
	Parameters() HandlerParameters
	// runs after validating request
	Initializer(req HandlerRequest[Req, Resp]) error
	// main handler runs after initialize
	Handler(req HandlerRequest[Req, Resp]) (Resp, error)
	// runs after sending back response
	Finalizer(req HandlerRequest[Req, Resp])
	// handles simulation mode
	Simulation(req HandlerRequest[Req, Resp]) (Resp, error)
}

type HandlerRequest[Req any, Resp any] struct {
	Title    string
	Core     requestCore.RequestCoreInterface
	Header   *libRequest.RequestHeader
	Request  *Req
	Response Resp
	W        webFramework.WebFramework
	Args     []any
	RespSent bool
	Builder  func(status int, rawResp []byte, headers map[string]string) (*Resp, error)
	// Tracing fields
	Span    trace.Span
	SpanCtx context.Context
}

// Tracing methods for HandlerRequest
func (hr *HandlerRequest[Req, Resp]) AddSpanAttribute(key, value string) {
	if hr.Span != nil && hr.Span.IsRecording() {
		hr.Span.SetAttributes(attribute.String(key, value))
	}
}

func (hr *HandlerRequest[Req, Resp]) AddSpanAttributes(attrs map[string]string) {
	if hr.Span != nil && hr.Span.IsRecording() {
		for k, v := range attrs {
			hr.Span.SetAttributes(attribute.String(k, v))
		}
	}
}

func (hr *HandlerRequest[Req, Resp]) AddSpanEvent(name string, attrs map[string]string) {
	if hr.Span != nil && hr.Span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		hr.Span.AddEvent(name, trace.WithAttributes(eventAttrs...))
	}
}

func (hr *HandlerRequest[Req, Resp]) RecordSpanError(err error, attrs map[string]string) {
	if hr.Span != nil && hr.Span.IsRecording() {
		var eventAttrs []attribute.KeyValue
		for k, v := range attrs {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
		hr.Span.RecordError(err, trace.WithAttributes(eventAttrs...))
	}
}

func (hr *HandlerRequest[Req, Resp]) StartChildSpan(name string, attrs map[string]string) (context.Context, trace.Span) {
	if hr.SpanCtx == nil {
		hr.SpanCtx = context.Background()
	}

	tm := libTracing.GetGlobalTracingManager()
	return tm.StartSpanWithAttributes(hr.SpanCtx, name, attrs)
}

func BaseHandler[Req any, Resp any, Handler HandlerInterface[Req, Resp]](
	core requestCore.RequestCoreInterface,
	handler Handler,
	simulation bool,
	args ...any,
) any {
	params := handler.Parameters()
	webFramework.AddServiceRegistrationLog(params.Title)
	return func(c context.Context) {
		start := time.Now()
		var w webFramework.WebFramework
		if params.SaveToRequest {
			w = libContext.InitContext(c)
		} else {
			w = libContext.InitContextNoAuditTrail(c)
		}
		libContext.AddWebLogs(w, params.Title, webFramework.HandlerLogTag)

		// Initialize tracing if enabled
		var span trace.Span
		var spanCtx context.Context
		if params.EnableTracing {
			spanName := params.TracingSpanName
			if spanName == "" {
				spanName = params.Title
			}

			tm := libTracing.GetGlobalTracingManager()
			spanCtx, span = tm.StartSpanWithAttributes(w.Ctx, spanName, map[string]string{
				"handler.title": params.Title,
				"handler.path":  params.Path,
			})

			// Add handler attributes
			if span != nil && span.IsRecording() {
				span.SetAttributes(
					attribute.String("handler.title", params.Title),
					attribute.String("handler.path", params.Path),
					attribute.Bool("handler.validate_header", params.ValidateHeader),
					attribute.Bool("handler.save_to_request", params.SaveToRequest),
				)
			}
		}

		trx := HandlerRequest[Req, Resp]{
			Title:   params.Title,
			Args:    args,
			Core:    core,
			W:       w,
			Span:    span,
			SpanCtx: spanCtx,
		}

		defer func() {
			Recovery(start, w, handler, params, trx, core)
		}()

		// Ensure span is ended
		defer func() {
			if span != nil {
				span.End()
			}
		}()

		if simulation {
			resp, header, errParse := libRequest.ParseRequest[Resp](
				trx.W,
				params.Body,
				params.ValidateHeader,
			)
			if errParse != nil {
				core.Responder().Error(trx.W, errParse)
				return
			}

			trx.Response = *resp
			trx.Header = header

			var err error
			trx.Response, err = handler.Simulation(trx)
			if err != nil {
				core.Responder().Error(trx.W, err)
				return
			}

			core.Responder().OK(trx.W, trx.Response)
		}

		var errParse error
		trx.Request, trx.Header, errParse = libRequest.ParseRequest[Req](
			trx.W,
			params.Body,
			params.ValidateHeader)
		if errParse != nil {
			core.Responder().Error(trx.W, errParse)
			return
		}
		webFramework.AddLog(w, webFramework.HandlerLogTag, slog.Any("request", trx.Request))

		if params.SaveToRequest {
			errInitRequest := core.RequestTools().InitRequest(trx.W, params.Title, params.Path)
			if errInitRequest != nil {
				core.Responder().Error(trx.W, errInitRequest)
				return
			}
		} else {
			w.Parser.SetLocal("reqLog", nil)
		}

		errInit := handler.Initializer(trx)
		webFramework.AddLog(w, webFramework.HandlerLogTag, slog.Any("initialize", errInit))
		if errInit != nil {
			core.Responder().Error(trx.W, errInit)
			return
		}

		var err error
		trx.Response, err = handler.Handler(trx)
		webFramework.AddLog(w, webFramework.HandlerLogTag, slog.Any("main-handler", err))
		if err != nil {
			core.Responder().Error(trx.W, err)
			return
		}
		webFramework.AddLog(w, webFramework.HandlerLogTag, slog.Any("response", trx.Response))
		if params.HasReceipt {
			receipt := trx.W.Parser.GetLocal("receipt")
			if receipt != nil {
				rc, ok := receipt.(*response.Receipt)
				if ok {
					core.Responder().OKWithReceipt(trx.W, trx.Response, rc)
					trx.RespSent = true
				} else {
					slog.Error("registered as handler with receipt, but receipt local was", slog.Any("receipt", fmt.Sprintf("%t", receipt)))
				}
			} else {
				slog.Error("registered as handler with receipt, but receipt local was absent")
			}
		}

		if params.FileResponse {
			attachment := trx.W.Parser.GetLocal("attachment")
			if attachment != nil {
				rc, ok := attachment.(*response.FileResponse)
				if ok {
					core.Responder().OKWithAttachment(trx.W, rc)
					trx.RespSent = true
				} else {
					slog.Error("registered as handler with attachment, but attachment local was", slog.Any("receipt", fmt.Sprintf("%t", attachment)))
				}
			} else {
				slog.Error("registered as handler with attachment, but attachment local was absent")
			}
		}

		if !trx.RespSent {
			core.Responder().OK(trx.W, trx.Response)
			trx.RespSent = true
		}
	}
}
