package handlers

import (
	"fmt"
	"net/http"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

type QueryHandlerType[Row any, Resp []Row] struct {
	Title        string
	Path         string
	Mode         libRequest.Type
	VerifyHeader bool
	Key          string
	Command      libQuery.QueryCommand
}

func (q QueryHandlerType[Row, Resp]) Parameters() (string, libRequest.Type, bool, bool, string) {
	return q.Title, q.Mode, q.VerifyHeader, false, q.Path
}
func (q QueryHandlerType[Row, Resp]) Initializer(req HandlerRequest[Row, Resp]) response.ErrorState {
	return nil
}
func (q QueryHandlerType[Row, Resp]) Handler(req HandlerRequest[Row, Resp]) (Resp, response.ErrorState) {
	anyArgs := []any{}
	for id := range q.Command.Args {
		anyArgs = append(anyArgs, q.Command.Args[id])
	}
	resp, err := libQuery.GetQuery[Row](
		q.Command.Command,
		req.Core.GetDB(),
		anyArgs...)
	if err != nil {
		return req.Response, err
	}
	switch q.Command.Type {
	case libQuery.QuerySingle:
		return Resp{resp[0]}, nil
	case libQuery.QueryAll:
		return resp, nil
	}
	return nil, response.Error(
		http.StatusInternalServerError,
		"COMMAND_TYPE_NOT_SUPPORTED",
		q.Command,
		fmt.Errorf("commandNotDefined"))
}
func (q QueryHandlerType[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, response.ErrorState) {
	return req.Response, nil
}
func (q QueryHandlerType[Req, Resp]) Finalizer(req HandlerRequest[Req, Resp]) {
}

func QueryHandler[Row any, Resp []Row](
	title, key, path string, queryMap map[string]libQuery.QueryCommand,
	core requestCore.RequestCoreInterface,
	mode libRequest.Type,
	validateHeader, simulation bool,
) any {
	return BaseHandler[Row, Resp, QueryHandlerType[Row, Resp]](core,
		QueryHandlerType[Row, Resp]{
			Mode:         mode,
			VerifyHeader: validateHeader,
			Title:        title,
			Key:          key,
			Command:      queryMap[key],
			Path:         path,
		},
		simulation)
}
