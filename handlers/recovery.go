package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libTracing"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/webFramework"
)

func Recovery[Req any, Resp any, Handler HandlerInterface[Req, Resp]](
	start time.Time,
	w webFramework.WebFramework,
	handler Handler,
	params HandlerParameters[Req, Resp],
	trx *HandlerRequest[Req, Resp],
	core requestCore.RequestCoreInterface,
	requestInserted bool,
	panicVal any,
) {

	elapsed := time.Since(start)
	trx.Duration = elapsed
	webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.String("elapsed", elapsed.String()))

	var panicErr error
	if panicVal != nil {
		if params.RecoveryHandler != nil {
			params.RecoveryHandler(panicVal)
		} else {
			slog.Error("panic recovered", slog.String("handler", params.Title), slog.Any("panic", panicVal))
			switch data := panicVal.(type) {
			case error:
				panicErr = errors.Join(data,
					libError.NewWithDescription(
						status.InternalServerError,
						response.SYSTEM_FAULT,
						"panic in %s",
						params.Title,
					))
			default:
				panicErr = libError.NewWithDescription(
					http.StatusInternalServerError,
					response.SYSTEM_FAULT,
					"panic in %s",
					params.Title)
			}
			trx.SetOutcome(panicErr, http.StatusInternalServerError)
		}
	}

	libTracing.TraceVoid(handler.Finalizer, *trx)
	webFramework.CollectLogTags(w, webFramework.HandlerLogTag)
	webFramework.CollectLogArrays(w, webFramework.HandlerLogTag)
	webFramework.CollectLogTags(w, webFramework.ErrorListLogTag)
	webFramework.CollectLogArrays(w, webFramework.ErrorListLogTag)
	webFramework.CollectLogArrays(w, CallApiLogEntry)
	for id := range params.LogTags {
		webFramework.CollectLogTags(w, params.LogTags[id])
	}
	for id := range params.LogArrays {
		webFramework.CollectLogArrays(w, params.LogArrays[id])
	}
	if params.Persistence != nil && requestInserted {
		if errUpdate := params.Persistence.Update(params.Path, trx); errUpdate != nil {
			slog.Error("request persistence update failed",
				slog.String("handler", params.Title),
				slog.String("path", params.Path),
				slog.Any("error", errUpdate))
		}
	}
	if panicVal != nil {
		if panicErr != nil {
			core.Responder().Error(w, panicErr)
		}
		panic(panicVal)
	}
}
