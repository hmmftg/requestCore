package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libLogger"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
	legacy "github.com/hmmftg/requestCore/webFramework"

	v2response "github.com/hmmftg/requestCore/v2/response"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// runLifecycle executes the parse → log → persist → init → handle → render
// sequence for a typed Endpoint. It returns an error if any step fails;
// the caller handles response writing for errors.
//
// This is fully typed — no type assertions, no carrier struct, no
// reflection. The Req and Resp types flow directly from Endpoint[Req, Resp].
func runLifecycle[Req, Resp any](
	endpoint *Endpoint[Req, Resp],
	respHandler *v2response.Handler,
	w legacy.WebFramework,
	ctx *v2wf.RequestContext,
	trx *HandlerRequest[Req, Resp],
	requestInserted *bool,
	start time.Time,
) error {
	// Parse the request through the root libRequest.ParseRequest.
	req, header, errParse := libRequest.ParseRequest[Req](w, endpoint.Body, endpoint.ValidateHeader)
	if errParse != nil {
		if wErr := respondErrorV2(respHandler, w, ctx, trx, endpoint.Title, errParse); wErr != nil {
			legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
			return wErr
		}
		legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("error", errParse))
		return errParse
	}
	trx.Header = header
	trx.Request = req

	w.Parser.SetLocal(libLogger.SlogRequestBody, trx.Request)
	legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("request", trx.Request))

	// ID parser (runs before initializer, used by resource registration
	// to parse URL parameters like resource IDs).
	if endpoint.idParser != nil {
		if errID := endpoint.idParser(ctx); errID != nil {
			legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("id-parse", errID))
			if wErr := respondErrorV2(respHandler, w, ctx, trx, endpoint.Title, errID); wErr != nil {
				legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
				return wErr
			}
			legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("error", errID))
			return errID
		}
	}

	// Persistence insert (best-effort, but failures abort).
	if endpoint.persistence != nil {
		if errInsert := endpoint.persistence.Insert(endpoint.Path, trx); errInsert != nil {
			if wErr := respondErrorV2(respHandler, w, ctx, trx, endpoint.Title, errInsert); wErr != nil {
				legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
				return wErr
			}
			legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("error", errInsert))
			return errInsert
		}
		*requestInserted = true
	}

	// Initializer.
	if endpoint.initializer != nil {
		if errInit := endpoint.initializer(trx); errInit != nil {
			legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("initialize", errInit))
			if wErr := respondErrorV2(respHandler, w, ctx, trx, endpoint.Title, errInit); wErr != nil {
				legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
				return wErr
			}
			legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("error", errInit))
			return errInit
		}
		legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("initialize", nil))
	}

	// Main handler — typed direct call, no type assertion.
	resp, err := endpoint.handler(trx.Request, trx)
	legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("main-handler", err))
	if err != nil {
		if wErr := respondErrorV2(respHandler, w, ctx, trx, endpoint.Title, err); wErr != nil {
			legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
			return wErr
		}
		legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("error", err))
		return err
	}
	trx.Response = resp
	legacy.AddLog(w, legacy.HandlerLogTag, slog.Any("response", resp))
	// Emit the mandatory <title>-req success AddLog entry containing the
	// parsed response. If the response implements slog.LogValuer, the
	// masked projection is used so response owners control what flows
	// into the Splunk transaction pipeline. The returned HTTP response
	// itself is never modified by this projection.
	legacy.AddLog(w, endpoint.Title+"-req", slog.Any("response", logValueForAddLog(resp)))

	// Render success through the v2 responder.
	if wErr := respondOKV2(respHandler, w, ctx, trx, endpoint.Title, resp); wErr != nil {
		legacy.AddLog(w, endpoint.Title+"-req-failed", slog.Any("write-error", wErr))
		return wErr
	}
	return nil
}

// logValueForAddLog returns the value to log for a parsed response in the
// mandatory <title>-req success AddLog entry. If the response implements
// slog.LogValuer, its LogValue projection is used so response-type owners
// can mask sensitive fields. A panic in LogValue is recovered and reported
// as a masking failure; the raw response is never logged as a fallback.
func logValueForAddLog(resp any) any {
	if lv, ok := resp.(slog.LogValuer); ok {
		var val slog.Value
		panicked := true
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("addlog: LogValue panic masked",
						slog.Any("panic", r))
				}
			}()
			val = lv.LogValue()
			panicked = false
		}()
		if panicked {
			// Masking failed; do not fall back to the raw response.
			return slog.StringValue("<masked: logvalue-panic>")
		}
		return val
	}
	return resp
}

// finalize runs the finalizer, collects logs, updates persistence
// best-effort, and handles panic conversion to a sanitized error response.
// logCompletion is the closure returned by libContext.AddWebLogs; it is
// invoked exactly once here to log elapsed+status and collect the
// HandlerLogTag tags/arrays.
func finalize[Req, Resp any](
	endpoint *Endpoint[Req, Resp],
	respHandler *v2response.Handler,
	w legacy.WebFramework,
	ctx *v2wf.RequestContext,
	trx *HandlerRequest[Req, Resp],
	start time.Time,
	requestInserted bool,
	panicVal any,
	logCompletion func(time.Time, int),
) {
	elapsed := time.Since(start)
	trx.Duration = elapsed

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
		trx.SetOutcome(panicErr, http.StatusInternalServerError)
	}

	// Finalizer (best-effort, documented guarantee: always runs).
	if endpoint.finalizer != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("finalizer panic",
						slog.String("handler", endpoint.Title),
						slog.Any("panic", r))
				}
			}()
			endpoint.finalizer(trx)
		}()
	}

	// Invoke the AddWebLogs completion closure exactly once. This logs
	// elapsed and status under HandlerLogTag and collects the tag's
	// tags/arrays into custom attributes. The status reflects the
	// committed response state (or 500 for panics).
	status := committedStatus(ctx)
	if status == 0 {
		status = http.StatusInternalServerError
	}
	if logCompletion != nil {
		logCompletion(start, status)
	}

	// Collect remaining log arrays/tags (error list, API calls, and
	// endpoint-specific tags). HandlerLogTag collection is performed by
	// the completion closure above.
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
	if endpoint.persistence != nil && requestInserted {
		if errUpdate := endpoint.persistence.Update(endpoint.Path, trx); errUpdate != nil {
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
			_ = respondErrorV2(respHandler, w, ctx, trx, endpoint.Title, panicErr)
		}
	}
}

// respondErrorV2 routes an error through the v2 response handler and records
// the outcome on the typed trx. It returns any transport write error so the
// caller can propagate it via mandatory AddLog.
func respondErrorV2[Req, Resp any](
	respHandler *v2response.Handler,
	w legacy.WebFramework,
	ctx *v2wf.RequestContext,
	trx *HandlerRequest[Req, Resp],
	title string,
	err error,
) error {
	if err == nil {
		return nil
	}
	if wErr := respHandler.Error(ctx, err); wErr != nil {
		// Transport write failed — record the write failure.
		legacy.AddLog(w, title+"-req-failed", slog.Any("write-error", wErr))
		trx.SetOutcome(wErr, http.StatusInternalServerError)
		return wErr
	}
	httpStatus := committedStatus(ctx)
	if httpStatus == 0 {
		httpStatus = http.StatusInternalServerError
	}
	trx.SetOutcome(err, httpStatus)
	return nil
}

// respondOKV2 renders a successful response through the v2 responder and
// records the outcome. It returns any transport write or renderer error.
func respondOKV2[Req, Resp any](
	respHandler *v2response.Handler,
	w legacy.WebFramework,
	ctx *v2wf.RequestContext,
	trx *HandlerRequest[Req, Resp],
	title string,
	resp Resp,
) error {
	if wErr := respHandler.OK(ctx, resp); wErr != nil {
		// Renderer encode or transport write failed.
		legacy.AddLog(w, title+"-req-failed", slog.Any("write-error", wErr))
		trx.SetOutcome(wErr, http.StatusInternalServerError)
		return wErr
	}
	httpStatus := committedStatus(ctx)
	if httpStatus == 0 {
		httpStatus = http.StatusOK
	}
	trx.SetOutcome(nil, httpStatus)
	trx.RespSent = true
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
