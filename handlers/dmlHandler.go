package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

func ExecDML(request libQuery.DmlModel, key, title string, w webFramework.WebFramework, core requestCore.RequestCoreInterface) (map[string]any, response.ErrorState) {
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

func PreControlDML(request libQuery.DmlModel, key, title string, w webFramework.WebFramework, core requestCore.RequestCoreInterface) response.ErrorState {
	preControl := request.PreControlCommands()
	for _, command := range preControl[key] {
		title := fmt.Sprintf("PreControl: %s", command.Name)
		core.RequestTools().LogStart(w, title, "Execute")
		_, errPreControl := command.ExecuteWithContext(
			w.Parser, w.Ctx, title, fmt.Sprintf("%s.%s", "preControl", command.Name), core.GetDB())
		if errPreControl != nil {
			return response.Errors(http.StatusInternalServerError, errPreControl.GetDescription(), title, errPreControl)
		}
	}
	return nil
}

func ExecuteDML(request libQuery.DmlModel, key, title string, w webFramework.WebFramework, core requestCore.RequestCoreInterface) (map[string]any, response.ErrorState) {
	dml := request.DmlCommands()
	resp := map[string]any{}
	for _, command := range dml[key] {
		title := fmt.Sprintf("Insert: %s", command.Name)
		core.RequestTools().LogStart(w, title, "Execute")
		result, errInsert := command.ExecuteWithContext(
			w.Parser, w.Ctx, title, fmt.Sprintf("%s.%s", "dml", command.Name), core.GetDB())
		if errInsert != nil {
			return nil, response.Errors(http.StatusInternalServerError, errInsert.GetDescription(), title, errInsert)
		}
		resp[command.Name] = result
	}
	return resp, nil
}

func FinalizeDML(request libQuery.DmlModel, key, title string, w webFramework.WebFramework, core requestCore.RequestCoreInterface) {
	finalize := request.FinalizeCommands()
	for _, command := range finalize[key] {
		title := fmt.Sprintf("Finalize: %s", command.Name)
		_, errFinalize := command.ExecuteWithContext(
			w.Parser, w.Ctx, title, title, core.GetDB())
		if errFinalize != nil {
			webFramework.AddLog(w, webFramework.HandlerLogTag,
				slog.Group("Error executing finalize command", slog.String("title", title), slog.Any("error", errFinalize)))
		}
	}
}

type DmlHandlerType[Req libQuery.DmlModel, Resp map[string]any] struct {
	Title           string
	Path            string
	Mode            libRequest.Type
	VerifyHeader    bool
	Key             string
	RecoveryHandler func(any)
}

func (h DmlHandlerType[Req, Resp]) Parameters() HandlerParameters {
	return HandlerParameters{h.Title, h.Mode, h.VerifyHeader, true, h.Path, false, h.RecoveryHandler, false, nil, nil}
}
func (h DmlHandlerType[Req, Resp]) Initializer(req HandlerRequest[Req, Resp]) response.ErrorState {
	return PreControlDML(*req.Request, h.Key, req.Title, req.W, req.Core)
}
func (h DmlHandlerType[Req, Resp]) Handler(req HandlerRequest[Req, Resp]) (Resp, response.ErrorState) {
	resp, err := ExecuteDML(*req.Request, h.Key, req.Title, req.W, req.Core)
	return resp, err
}
func (h DmlHandlerType[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, response.ErrorState) {
	return req.Response, nil
}
func (h DmlHandlerType[Req, Resp]) Finalizer(req HandlerRequest[Req, Resp]) {
	if req.RespSent {
		FinalizeDML(*req.Request, h.Key, req.Title, req.W, req.Core)
	}
}

func DmlHandler[Req libQuery.DmlModel](
	core requestCore.RequestCoreInterface,
	handler DmlHandlerType[Req, map[string]any],
	simulation bool,
) any {
	return BaseHandler(core, handler, simulation)
}
