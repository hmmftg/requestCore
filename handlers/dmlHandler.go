package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

func Dml[Req libQuery.DmlModel](
	title, key string,
	core requestCore.RequestCoreInterface,
) any {
	return DmlHandler[Req](title, key, core, libRequest.JSON, true)
}

func ExecDML(request libQuery.DmlModel, key, title string, w webFramework.WebFramework, core requestCore.RequestCoreInterface) (map[string]any, *response.ErrorState) {
	errPreControl := PreControlDML(request, key, title, w, core)
	if errPreControl != nil {
		return nil, errPreControl
	}
	resp, errExec := ExecuteDML(request, key, title, w, core)
	if errExec != nil {
		return nil, errExec
	}

	FinalizeDML(request, key, title, w, core)
	return resp, nil
}

func PreControlDML(request libQuery.DmlModel, key, title string, w webFramework.WebFramework, core requestCore.RequestCoreInterface) *response.ErrorState {
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

func ExecuteDML(request libQuery.DmlModel, key, title string, w webFramework.WebFramework, core requestCore.RequestCoreInterface) (map[string]any, *response.ErrorState) {
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

func FinalizeDML(request libQuery.DmlModel, key, title string, w webFramework.WebFramework, core requestCore.RequestCoreInterface) {
	finalize := request.FinalizeCommands()
	for _, command := range finalize[key] {
		_, errFinalize := command.ExecuteWithContext(
			w.Ctx, title, fmt.Sprintf("%s.%s", "finalize", command.Name), core.GetDB())
		if errFinalize != nil {
			log.Printf("Error executing finalize command: %s=>%v", command.Name, errFinalize)
		}
	}
}

type DmlHandlerType[Req libQuery.DmlModel, Resp map[string]any] struct {
	Title        string
	Path         string
	Mode         libRequest.Type
	VerifyHeader bool
	Key          string
}

func (h DmlHandlerType[Req, Resp]) Parameters() (string, libRequest.Type, bool, bool, string) {
	return h.Title, h.Mode, h.VerifyHeader, true, h.Path
}
func (h DmlHandlerType[Req, Resp]) Initializer(req HandlerRequest[Req, Resp]) *response.ErrorState {
	return PreControlDML(*req.Request, h.Key, req.Title, req.W, req.Core)
}
func (h DmlHandlerType[Req, Resp]) Handler(req HandlerRequest[Req, Resp]) (Resp, *response.ErrorState) {
	resp, err := ExecuteDML(*req.Request, h.Key, req.Title, req.W, req.Core)
	return resp, err
}
func (h DmlHandlerType[Req, Resp]) Finalizer(req HandlerRequest[Req, Resp]) {
	FinalizeDML(*req.Request, h.Key, req.Title, req.W, req.Core)
}

func DmlHandlerNew[Req libQuery.DmlModel](
	title, key string,
	core requestCore.RequestCoreInterface,
	mode libRequest.Type,
	validateHeader bool,
) any {
	return BaseHandler[Req, map[string]any, DmlHandlerType[Req, map[string]any]](core, DmlHandlerType[Req, map[string]any]{Mode: mode, VerifyHeader: validateHeader})
}

func DmlHandler[Req libQuery.DmlModel](
	title, key string,
	core requestCore.RequestCoreInterface,
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
