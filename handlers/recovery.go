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
	params HandlerParameters,
	trx HandlerRequest[Req, Resp],
	core requestCore.RequestCoreInterface,
) {
	elapsed := time.Since(start)
	webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.String("elapsed", elapsed.String()))
	libTracing.TraceVoid(handler.Finalizer, trx)
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
	if r := recover(); r != nil {
		if params.RecoveryHandler != nil {
			params.RecoveryHandler(r)
		} else {
			switch data := r.(type) {
			case error:
				core.Responder().Error(w,
					errors.Join(data,
						libError.NewWithDescription(
							status.InternalServerError,
							response.SYSTEM_FAULT,
							"error in %s",
							params.Title,
						)))
			default:
				core.Responder().Error(w,
					libError.NewWithDescription(
						http.StatusInternalServerError,
						response.SYSTEM_FAULT,
						"error in %s=> %+v", params.Title, data),
				)
			}
		}
		panic(r)
	}
}
