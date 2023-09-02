package requestCore

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

func Dml[Req libQuery.DmlModel](
	title, key string,
	core RequestCoreInterface,
) any {
	return DmlHandler[Req](title, key, core, libRequest.JSON, true)
}

func DmlHandler[Req libQuery.DmlModel](
	title, key string,
	core RequestCoreInterface,
	mode libRequest.Type,
	validateHeader bool,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		defer func() {
			if r := recover(); r != nil {
				core.Responder().HandleErrorState(libError.Join(r.(error), "error in Dml"), http.StatusInternalServerError, response.SYSTEM_FAULT, response.SYSTEM_FAULT_DESC, c)
				panic(r)
			}
		}()
		w := libContext.InitContext(c)
		code, desc, arrayErr, request, reqLog, err := libRequest.Req[Req, libRequest.RequestHeader](w, mode, validateHeader)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}

		w.Parser.SetLocal("reqLog", &reqLog)
		method := title
		reqLog.Incoming = request
		u, _ := url.Parse(w.Parser.GetPath())
		code, result, err := core.RequestTools().Initialize(w, method, u.Path, &reqLog)
		if err != nil {
			core.Responder().HandleErrorState(err, code, result["desc"], result["message"], c)
			return
		}

		preControl := request.PreControlCommands()
		for _, command := range preControl[key] {
			core.RequestTools().LogStart(w, fmt.Sprintf("PreControl: %s", command.Name), "Execute")
			_, errPreControl := command.ExecuteWithContext(w.Ctx, title, fmt.Sprintf("%s.%s", "preControl", command.Name), core.GetDB())
			if errPreControl != nil {
				core.Responder().HandleErrorState(libError.Join(errPreControl, "PreControl"), http.StatusBadRequest, errPreControl.Description, errPreControl.Message, c)
				return
			}
		}
		dml := request.DmlCommands()
		resp := map[string]any{}
		for _, command := range dml[key] {
			core.RequestTools().LogStart(w, fmt.Sprintf("Insert: %s", command.Name), "Execute")
			result, errInsert := command.ExecuteWithContext(w.Ctx, title, fmt.Sprintf("%s.%s", "dml", command.Name), core.GetDB())
			if errInsert != nil {
				core.Responder().HandleErrorState(libError.Join(errInsert, "Insert"), http.StatusBadRequest, errInsert.Description, errInsert.Message, c)
				return
			}
			resp[command.Name] = result
		}

		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, c)

		finalize := request.FinalizeCommands()
		for _, command := range finalize[key] {
			_, errFinalize := command.ExecuteWithContext(w.Ctx, title, fmt.Sprintf("%s.%s", "finalize", command.Name), core.GetDB())
			if errFinalize != nil {
				log.Printf("Error executing finalize command: %s=>%v", command.Name, errFinalize)
			}
		}
	}
}
