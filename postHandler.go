package requestCore

import (
	"context"
	"log"
	"net/http"
	"net/url"

	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
)

func PostHandler[Req libQuery.RecordDataDml](title string,
	core RequestCoreInterface,
	hasInitializer bool,
	finalizer func(request Req, c any),
	args ...any,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		w := libContext.InitContext(c)
		code, desc, arrayErr, request, reqLog, err := libRequest.GetRequest[Req](w, true)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}

		if hasInitializer {
			w.Parser.SetLocal("reqLog", &reqLog)
			method := title
			reqLog.Incoming = request
			u, _ := url.Parse(w.Parser.GetPath())
			code, result, err := core.RequestTools().Initialize(w, method, u.Path, &reqLog)
			if err != nil {
				core.Responder().HandleErrorState(err, code, result["desc"], result["message"], c)
				return
			}
		}

		desc, err = request.Filler(w.Parser.GetHttpHeader(), core.GetDB(), w.Parser.GetArgs(args...))
		if err != nil {
			core.Responder().HandleErrorState(err, http.StatusInternalServerError, desc, "error in Filler", c)
			return
		}

		code, desc, err = request.CheckDuplicate(core.GetDB())
		if err != nil {
			core.Responder().HandleErrorState(libError.Join(err, "error in CheckDuplicate"), code, desc, "", c)
			return
		}

		code, desc, err = request.PreControl(core.GetDB())
		if err != nil {
			core.Responder().HandleErrorState(libError.Join(err, "error in PreControl"), code, desc, "", c)
			return
		}

		resp, code, desc, err := request.Post(core.GetDB(), w.Parser.GetArgs(args...))
		if err != nil {
			core.Responder().HandleErrorState(libError.Join(err, "error in Post"), code, desc, "", c)
			return
		}

		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, c)
		if finalizer != nil {
			finalizer(request, c)
		}
	}
}
