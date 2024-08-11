package handlers

import (
	"log"
	"net/http"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

type RowTranslator[Row, Resp any] interface {
	Translate([]Row, HandlerRequest[Row, Resp]) (Resp, response.ErrorState)
}

type QuerySingleTransformer[Row any, Resp []Row] struct {
}

func (s QuerySingleTransformer[Row, Resp]) Translate(rows []Row, req HandlerRequest[Row, Resp]) (Resp, response.ErrorState) {
	return Resp{rows[0]}, nil
}

type QueryAllTransformer[Row any, Resp []Row] struct {
}

func (s QueryAllTransformer[Row, Resp]) Translate(rows []Row, req HandlerRequest[Row, Resp]) (Resp, response.ErrorState) {
	return rows, nil
}

type QueryHandlerType[Row, Resp any] struct {
	Title           string
	Path            string
	Mode            libRequest.Type
	VerifyHeader    bool
	Key             string
	Command         libQuery.QueryCommand
	Translator      RowTranslator[Row, Resp]
	RecoveryHandler func(any)
}

func (q QueryHandlerType[Row, Resp]) Parameters() HandlerParameters {
	return HandlerParameters{q.Title, q.Mode, q.VerifyHeader, false, q.Path, false, q.RecoveryHandler, false}
}
func (q QueryHandlerType[Row, Resp]) Initializer(req HandlerRequest[Row, Resp]) response.ErrorState {
	return nil
}
func (q QueryHandlerType[Row, Resp]) Handler(req HandlerRequest[Row, Resp]) (Resp, response.ErrorState) {
	anyArgs := []any{}
	for id := range q.Command.Args {
		_, val, err := libQuery.GetFormTagValue(q.Command.Args[id], req.Request)
		if err != nil {
			return req.Response, response.Error(
				http.StatusInternalServerError,
				"COMMAND_ARGUMENT_ERROR",
				q.Command,
				err)
		}
		anyArgs = append(anyArgs, *val)
	}
	rows, err := libQuery.GetQuery[Row](
		q.Command.Command,
		req.Core.GetDB(),
		anyArgs...)
	if err != nil {
		return req.Response, err
	}
	resp, err := q.Translator.Translate(rows, req)
	if err != nil {
		return req.Response, err
	}

	return resp, nil
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
	recoveryHandler func(any),
) any {
	command := queryMap[key]
	switch command.Type {
	case libQuery.QuerySingle:
		return BaseHandler(core,
			QueryHandlerType[Row, Resp]{
				Mode:            mode,
				VerifyHeader:    validateHeader,
				Title:           title,
				Key:             key,
				Command:         command,
				Path:            path,
				Translator:      QuerySingleTransformer[Row, Resp]{},
				RecoveryHandler: recoveryHandler,
			},
			simulation)
	case libQuery.QueryAll:
		return BaseHandler(core,
			QueryHandlerType[Row, Resp]{
				Mode:            mode,
				VerifyHeader:    validateHeader,
				Title:           title,
				Key:             key,
				Command:         command,
				Path:            path,
				Translator:      QueryAllTransformer[Row, Resp]{},
				RecoveryHandler: recoveryHandler,
			},
			simulation)
	default:
		log.Fatalln("invalid command type", command.Type)
		return nil
	}
}

func QueryHandlerWithTransform[Row, Resp any](
	title, key, path string, queryMap map[string]libQuery.QueryCommand,
	core requestCore.RequestCoreInterface,
	mode libRequest.Type,
	validateHeader, simulation bool,
	recoveryHandler func(any),
	translator RowTranslator[Row, Resp],
) any {
	command := queryMap[key]
	command.Type = libQuery.Transforms
	return BaseHandler(core,
		QueryHandlerType[Row, Resp]{
			Mode:            mode,
			VerifyHeader:    validateHeader,
			Title:           title,
			Key:             key,
			Command:         command,
			Path:            path,
			Translator:      translator,
			RecoveryHandler: recoveryHandler,
		},
		simulation)
}
