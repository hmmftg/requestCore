package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libLogger"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/libTracing"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
	legacy "github.com/hmmftg/requestCore/webFramework"

	v2response "github.com/hmmftg/requestCore/v2/response"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// buildEndpointHandler constructs a routing.Handler that runs the full v2
// BaseHandler lifecycle for the given Endpoint. The core argument may be a
// requestCore.RequestCoreInterface or nil; when nil, legacy infrastructure
// is unavailable and persistence/tracing that requires it is skipped.
func buildEndpointHandler(
	core any,
	respHandler *v2response.Handler,
	endpoint *Endpoint,
	args []any,
) func(ctx *v2wf.RequestContext) error {
	if respHandler == nil {
		// Install a handler with a default fallback so the registry
		// never encounters a fallback-less state. This prevents
		// silent failures when respHandler is not explicitly configured.
		reg := v2response.NewRegistry(nil)
		reg.SetFallback(v2response.LegacyFallback(response.WebHanlder{}))
		respHandler = v2response.NewHandler(reg, nil, response.WebHanlder{})
	}
	return func(ctx *v2wf.RequestContext) (err error) {
		start := time.Now()

		// Initialize the legacy WebFramework from the v2 RequestContext's
		// LegacyContext. This preserves the v1 parser/tracing pipeline.
		var w legacy.WebFramework
		if ctx.LegacyContext != nil {
			if endpoint.persistence != nil {
				w = libContext.InitContext(ctx.LegacyContext)
			} else {
				w = libContext.InitContextNoAuditTrail(ctx.LegacyContext)
			}
		} else {
			// Fall back to the v2-carried legacy framework (tests).
			w = ctx.Legacy
		}
		// Ensure the v2 parser and legacy parser are the same instance.
		if ctx.Legacy.Parser == nil && w.Parser != nil {
			ctx.Legacy.Parser = w.Parser
		}
		libContext.AddWebLogs(w, endpoint.Title, legacy.HandlerLogTag)

		// Initialize tracing if enabled. The span is ended via defer
		// so it always completes, even on panic.
		var span trace.Span
		var spanCtx context.Context
		if endpoint.EnableTracing {
			spanName := endpoint.TracingSpanName
			if spanName == "" {
				spanName = endpoint.Title
			}
			tm := libTracing.GetGlobalTracingManager()
			if tm != nil {
				spanCtx, span = tm.StartSpanWithAttributes(w.Ctx, spanName, map[string]string{
					"handler.title": endpoint.Title,
					"handler.path":  endpoint.Path,
				})
			}
			if span != nil && span.IsRecording() {
				span.SetAttributes(
					attribute.String("handler.title", endpoint.Title),
					attribute.String("handler.path", endpoint.Path),
					attribute.Bool("handler.validate_header", endpoint.ValidateHeader),
					attribute.Bool("handler.has_persistence", endpoint.persistence != nil),
				)
			}
			defer func() {
				if span != nil {
					if err != nil {
						span.RecordError(err)
						span.SetAttributes(attribute.Int("handler.error_status", committedStatus(ctx)))
					}
					span.SetAttributes(attribute.Int("handler.status", committedStatus(ctx)))
					span.End()
				}
			}()
		}

		// Build the typed trx carrier via the endpoint's captured closure.
		trxCarrier := endpoint.buildCarrier(core, w, ctx, args, span, spanCtx)

		// Panic recovery + finalization + log collection.
		var requestInserted bool
		var panicVal any
		func() {
			defer func() {
				panicVal = recover()
			}()
			err = runLifecycle(endpoint, respHandler, w, ctx, trxCarrier, &requestInserted, start)
		}()

		finalize(endpoint, respHandler, w, ctx, trxCarrier, start, requestInserted, panicVal)

		// If a panic occurred, it was converted to a sanitized error
		// response in finalize. Return nil so the router does not
		// double-write. If the lifecycle returned an error and nothing
		// was committed, the router registry will handle it.
		if panicVal != nil {
			return nil
		}
		if err != nil && !ctx.Committed() {
			return err
		}
		return nil
	}
}

// runLifecycle executes the parse → log → persist → init → handle → render
// sequence. It returns an error if any step fails; the caller handles
// response writing for errors.
func runLifecycle(
	endpoint *Endpoint,
	respHandler *v2response.Handler,
	w legacy.WebFramework,
	ctx *v2wf.RequestContext,
	trxCarrier *endpointTrxCarrier,
	requestInserted *bool,
	start time.Time,
) error {
	// Parse the request through the root libRequest.ParseRequest using
	// the typed closure captured in the endpoint.
	reqPtr, header, errParse := trxCarrier.parseRequest(w)
	if errParse != nil {
		if wErr := respondErrorV2(respHandler, w, ctx, trxCarrier, endpoint.Title, errParse); wErr != nil {
			legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
			return wErr
		}
		legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("error", errParse))
		return errParse
	}
	trxCarrier.setHeader(header)
	trxCarrier.setRequest(reqPtr)

	w.Parser.SetLocal(libLogger.SlogRequestBody, reqPtr)
	legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("request", reqPtr))

	// ID parser (runs before initializer, used by resource registration
	// to parse URL parameters like resource IDs).
	if endpoint.idParser != nil {
		if errID := endpoint.idParser(ctx); errID != nil {
			legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("id-parse", errID))
			if wErr := respondErrorV2(respHandler, w, ctx, trxCarrier, endpoint.Title, errID); wErr != nil {
				legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
				return wErr
			}
			legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("error", errID))
			return errID
		}
	}

	// Persistence insert (best-effort, but failures abort).
	if endpoint.persistence != nil && trxCarrier.insertPersistence != nil {
		if errInsert := trxCarrier.insertPersistence(trxCarrier); errInsert != nil {
			if wErr := respondErrorV2(respHandler, w, ctx, trxCarrier, endpoint.Title, errInsert); wErr != nil {
				legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
				return wErr
			}
			legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("error", errInsert))
			return errInsert
		}
		*requestInserted = true
	}

	// Initializer.
	if trxCarrier.runInitializer != nil {
		if errInit := trxCarrier.runInitializer(trxCarrier); errInit != nil {
			legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("initialize", errInit))
			if wErr := respondErrorV2(respHandler, w, ctx, trxCarrier, endpoint.Title, errInit); wErr != nil {
				legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
				return wErr
			}
			legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("error", errInit))
			return errInit
		}
		legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("initialize", nil))
	}

	// Main handler.
	resp, err := trxCarrier.runHandler(trxCarrier)
	legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("main-handler", err))
	if err != nil {
		if wErr := respondErrorV2(respHandler, w, ctx, trxCarrier, endpoint.Title, err); wErr != nil {
			legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
			return wErr
		}
		legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("error", err))
		return err
	}
	trxCarrier.setResponse(resp)
	legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("response", resp))

	// Render success through the v2 responder.
	if wErr := respondOKV2(respHandler, w, ctx, trxCarrier, endpoint.Title, resp); wErr != nil {
		legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
		return wErr
	}
	return nil
}

// finalize runs the finalizer, collects logs, updates persistence
// best-effort, and handles panic conversion to a sanitized error response.
func finalize(
	endpoint *Endpoint,
	respHandler *v2response.Handler,
	w legacy.WebFramework,
	ctx *v2wf.RequestContext,
	trxCarrier *endpointTrxCarrier,
	start time.Time,
	requestInserted bool,
	panicVal any,
) {
	elapsed := time.Since(start)
	trxCarrier.setDuration(elapsed)
	legacy.AddLogTag(w, legacy.HandlerLogTag, slog.String("elapsed", elapsed.String()))

	// Convert panic to a sanitized error. The recovery handler is
	// notification-only and must not suppress the standard error
	// response, outcome, persistence update, or AddLog failure path.
	var panicErr error
	if panicVal != nil {
		slog.Error("panic recovered",
			slog.String("handler", endpoint.Title),
			slog.Any("panic", panicVal))
		// Notify the optional recovery handler (extension hook only).
		if endpoint.recoveryHandler != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("recovery handler panic",
							slog.String("handler", endpoint.Title),
							slog.Any("panic", r))
					}
				}()
				endpoint.recoveryHandler(panicVal)
			}()
		}
		switch data := panicVal.(type) {
		case error:
			panicErr = errors.Join(data,
				libError.NewWithDescription(
					status.InternalServerError,
					response.SystemFault,
					"panic in %s",
					endpoint.Title,
				))
		default:
			panicErr = libError.NewWithDescription(
				http.StatusInternalServerError,
				response.SystemFault,
				"panic in %s",
				endpoint.Title)
		}
		trxCarrier.setOutcome(panicErr, http.StatusInternalServerError)
	}

	// Finalizer (best-effort, documented guarantee: always runs).
	if trxCarrier.runFinalizer != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("finalizer panic",
						slog.String("handler", endpoint.Title),
						slog.Any("panic", r))
				}
			}()
			trxCarrier.runFinalizer(trxCarrier)
		}()
	}

	// Collect logs into custom attributes for the Splunk pipeline.
	legacy.CollectLogTags(w, legacy.HandlerLogTag)
	legacy.CollectLogArrays(w, legacy.HandlerLogTag)
	legacy.CollectLogTags(w, legacy.ErrorListLogTag)
	legacy.CollectLogArrays(w, legacy.ErrorListLogTag)
	legacy.CollectLogArrays(w, CallAPILogEntry)
	for _, tag := range endpoint.LogTags {
		legacy.CollectLogTags(w, tag)
	}
	for _, arr := range endpoint.LogArrays {
		legacy.CollectLogArrays(w, arr)
	}

	// Persistence update (best-effort).
	if endpoint.persistence != nil && requestInserted && trxCarrier.updatePersistence != nil {
		if errUpdate := trxCarrier.updatePersistence(trxCarrier); errUpdate != nil {
			slog.Error("request persistence update failed",
				slog.String("handler", endpoint.Title),
				slog.String("path", endpoint.Path),
				slog.Any("error", errUpdate))
		}
	}

	// If a panic occurred, emit a mandatory AddLog failure and write a
	// sanitized error response (if not already committed).
	if panicVal != nil && panicErr != nil {
		legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("error", panicErr))
		if !ctx.Committed() {
			_ = respondErrorV2(respHandler, w, ctx, trxCarrier, endpoint.Title, panicErr)
		}
	}
}

// respondErrorV2 routes an error through the v2 response handler and records
// the outcome on the typed trx. It returns any transport write error so the
// caller can propagate it via mandatory AddLog.
func respondErrorV2(respHandler *v2response.Handler, w legacy.WebFramework, ctx *v2wf.RequestContext, trxCarrier *endpointTrxCarrier, title string, err error) error {
	if err == nil {
		return nil
	}
	if wErr := respHandler.Error(ctx, err); wErr != nil {
		// Transport write failed — record the write failure.
		legacy.AddLog(w, title+"-req-failed", slog.Any("write-error", wErr))
		trxCarrier.setOutcome(wErr, http.StatusInternalServerError)
		return wErr
	}
	httpStatus := committedStatus(ctx)
	if httpStatus == 0 {
		httpStatus = http.StatusInternalServerError
	}
	trxCarrier.setOutcome(err, httpStatus)
	return nil
}

// respondOKV2 renders a successful response through the v2 responder and
// records the outcome. It returns any transport write or renderer error.
func respondOKV2(respHandler *v2response.Handler, w legacy.WebFramework, ctx *v2wf.RequestContext, trxCarrier *endpointTrxCarrier, title string, resp any) error {
	if wErr := respHandler.OK(ctx, resp); wErr != nil {
		// Renderer encode or transport write failed.
		legacy.AddLog(w, title+"-req-failed", slog.Any("write-error", wErr))
		trxCarrier.setOutcome(wErr, http.StatusInternalServerError)
		return wErr
	}
	httpStatus := committedStatus(ctx)
	if httpStatus == 0 {
		httpStatus = http.StatusOK
	}
	trxCarrier.setOutcome(nil, httpStatus)
	trxCarrier.markRespSent()
	return nil
}

// committedStatus returns the HTTP status from the v2 commit state, or 0
// if the response has not been committed. This replaces the legacy
// response.LastHTTPStatus local for v2 committed status tracking.
func committedStatus(ctx *v2wf.RequestContext) int {
	if ctx == nil {
		return 0
	}
	if cs := ctx.CommitState(); cs != nil {
		return cs.Status()
	}
	return 0
}

// CallAPILogEntry mirrors the v1 handlers.CallAPILogEntry constant so v2
// handlers can collect API-call log arrays in the finalizer without
// importing the v1 handlers package directly.
const CallAPILogEntry string = "ApiCall"

// endpointTrxCarrier is a generic-free carrier that holds a pointer to the
// typed HandlerRequest[Req, Resp]. It is populated by the endpoint's
// captured closure so the lifecycle can mutate typed state without exposing
// generics on the non-generic Endpoint.
type endpointTrxCarrier struct {
	trx any // *HandlerRequest[Req, Resp]

	// typed accessors set by the endpoint closure.
	setHeader         func(*libRequest.RequestHeader)
	setRequest        func(any)
	setResponse       func(any)
	setOutcome        func(error, int)
	setDuration       func(time.Duration)
	markRespSent      func()
	parseRequest      func(legacy.WebFramework) (any, *libRequest.RequestHeader, error)
	insertPersistence func(*endpointTrxCarrier) error
	updatePersistence func(*endpointTrxCarrier) error
	runInitializer    func(*endpointTrxCarrier) error
	runHandler        func(*endpointTrxCarrier) (any, error)
	runFinalizer      func(*endpointTrxCarrier)
}
