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
	"github.com/hmmftg/requestCore/webFramework"
)

func Dml[Req libQuery.DmlModel](
	title, key string,
	core RequestCoreInterface,
) any {
	return DmlHandler[Req](title, key, core, libRequest.JSON, true)
}

func PreControlDML(request libQuery.DmlModel, key, title string, w webFramework.WebFramework, core RequestCoreInterface) *response.ErrorState {
	preControl := request.PreControlCommands()
	for _, command := range preControl[key] {
		core.RequestTools().LogStart(w, fmt.Sprintf("PreControl: %s", command.Name), "Execute")
		_, errPreControl := command.ExecuteWithContext(
			w.Ctx, title, fmt.Sprintf("%s.%s", "preControl", command.Name), core.GetDB())
		if errPreControl != nil {
			return errPreControl
		}
	}
	return nil
}

func ExecuteDML(request libQuery.DmlModel, key, title string, w webFramework.WebFramework, core RequestCoreInterface) (map[string]any, *response.ErrorState) {
	dml := request.DmlCommands()
	resp := map[string]any{}
	for _, command := range dml[key] {
		core.RequestTools().LogStart(w, fmt.Sprintf("Insert: %s", command.Name), "Execute")
		result, errInsert := command.ExecuteWithContext(
			w.Ctx, title, fmt.Sprintf("%s.%s", "dml", command.Name), core.GetDB())
		if errInsert != nil {
			return nil, errInsert
		}
		resp[command.Name] = result
	}
	return resp, nil
}

func FinalizeDML(request libQuery.DmlModel, key, title string, w webFramework.WebFramework, core RequestCoreInterface) {
	finalize := request.FinalizeCommands()
	for _, command := range finalize[key] {
		_, errFinalize := command.ExecuteWithContext(
			w.Ctx, title, fmt.Sprintf("%s.%s", "finalize", command.Name), core.GetDB())
		if errFinalize != nil {
			log.Printf("Error executing finalize command: %s=>%v", command.Name, errFinalize)
		}
	}
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
			w := libContext.InitContext(c)
			if r := recover(); r != nil {
				core.Responder().HandleErrorState(
					libError.Join(r.(error), "error in Dml"),
					http.StatusInternalServerError,
					response.SYSTEM_FAULT,
					response.SYSTEM_FAULT_DESC,
					w)
				panic(r)
			}
		}()
		w := libContext.InitContext(c)
		code, desc, arrayErr, request, reqLog, err := libRequest.Req[
			Req, libRequest.RequestHeader,
		](w, mode, validateHeader)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, w)
			return
		}

		w.Parser.SetLocal("reqLog", &reqLog)
		method := title
		reqLog.Incoming = request
		u, _ := url.Parse(w.Parser.GetPath())
		code, result, err := core.RequestTools().Initialize(w, method, u.Path, &reqLog)
		if err != nil {
			core.Responder().HandleErrorState(err, code, result["desc"], result["message"], w)
			return
		}

		errPreControl := PreControlDML(request, key, title, w, core)
		if errPreControl != nil {
			core.Responder().HandleErrorState(
				libError.Join(errPreControl, "PreControl"),
				http.StatusBadRequest,
				errPreControl.Description,
				errPreControl.Message,
				w)
			return
		}

		resp, errExec := ExecuteDML(request, key, title, w, core)
		if errExec != nil {
			core.Responder().HandleErrorState(
				libError.Join(errExec, "Execute"),
				http.StatusInternalServerError,
				errExec.Description,
				errExec.Message,
				w)
			return
		}

		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, w)

		FinalizeDML(request, key, title, w, core)
	}
}
